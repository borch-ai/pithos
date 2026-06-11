package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func findWorkspaceRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Abs(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Abs(wd)
}

func validateTemplate(templatePath string) error {
	//nolint:gosec // script runs in controlled local development/testing environment
	file, err := os.Open(templatePath)
	if err != nil {
		return fmt.Errorf("failed to open template file '%s': %w", templatePath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	hasTopLevel := false
	hasProposedChanges := false
	hasVerificationPlan := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# plan: Task ") {
			hasTopLevel = true
		}
		if line == "## Proposed Changes" {
			hasProposedChanges = true
		}
		if line == "## Verification Plan" {
			hasVerificationPlan = true
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading template file: %w", err)
	}

	if !hasTopLevel {
		return fmt.Errorf("template file is missing required '# plan: Task ' heading")
	}
	if !hasProposedChanges {
		return fmt.Errorf("template file is missing required '## Proposed Changes' heading")
	}
	if !hasVerificationPlan {
		return fmt.Errorf("template file is missing required '## Verification Plan' heading")
	}

	return nil
}

// Extract plan metadata (status and headings)
func parsePlanMetadata(filePath string) (bool, bool, bool, bool, error) {
	//nolint:gosec // script runs in controlled local development/testing environment
	file, err := os.Open(filePath)
	if err != nil {
		return false, false, false, false, err
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	hasTopLevelHeader := false
	hasProposedChanges := false
	hasVerificationPlan := false
	isCompleted := false

	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(trimmed, "# plan: Task ") {
			hasTopLevelHeader = true
		}
		if strings.Contains(trimmed, "**Status:**") {
			if strings.Contains(strings.ToLower(trimmed), "completed") {
				isCompleted = true
			}
		}
		if trimmed == "## Proposed Changes" {
			hasProposedChanges = true
		}
		if trimmed == "## Verification Plan" {
			hasVerificationPlan = true
		}
	}
	return hasTopLevelHeader, hasProposedChanges, hasVerificationPlan, isCompleted, scanner.Err()
}

// Helper to determine the action for a given line
func determineAction(line string) (string, error) {
	switch {
	case strings.Contains(line, "[NEW]"):
		return "NEW", nil
	case strings.Contains(line, "[MODIFY]"):
		return "MODIFY", nil
	case strings.Contains(line, "[DELETE]"):
		return "DELETE", nil
	default:
		return "", fmt.Errorf("file link heading does not contain action tag [NEW], [MODIFY], or [DELETE]")
	}
}

// Helper to validate a single file link match
func validateFileLink(line string, label string, urlStr string, isCompleted bool, workspaceRoot string) error {
	// 1. Must use file:/// (three slashes for absolute path)
	if !strings.HasPrefix(urlStr, "file:///") {
		return fmt.Errorf("file link '%s' must use 'file:///' scheme with absolute path", urlStr)
	}

	// Extract the absolute path from urlStr
	filePathPart := strings.TrimPrefix(urlStr, "file://")
	cleanPath := filepath.Clean(filePathPart)

	// 2. Must resolve to valid path inside workspace root
	rel, err := filepath.Rel(workspaceRoot, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("file path '%s' resolves outside the workspace root '%s'", cleanPath, workspaceRoot)
	}

	// 3. Label must match the base name of the path
	expectedBase := filepath.Base(cleanPath)
	if label != expectedBase {
		return fmt.Errorf("link label '[%s]' does not match the actual file basename '%s'", label, expectedBase)
	}

	// 4. Action determination
	action, err := determineAction(line)
	if err != nil {
		return err
	}

	// 5. Check existence based on status and action
	switch action {
	case "MODIFY":
		if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
			return fmt.Errorf("modified file '%s' does not exist on disk", rel)
		}
	case "NEW":
		if isCompleted {
			if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
				return fmt.Errorf("completed plan references new file '%s' which does not exist on disk", rel)
			}
		}
	}

	return nil
}

// Validate file links in headings
func validateFileLinks(filePath string, isCompleted bool, workspaceRoot string) []error {
	var errs []error
	//nolint:gosec // script runs in controlled local development/testing environment
	file, err := os.Open(filePath)
	if err != nil {
		return []error{fmt.Errorf("failed to open plan file for link check: %w", err)}
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	var lineNum int

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") {
			matches := linkRegex.FindAllStringSubmatch(line, -1)
			isActionHeading := strings.Contains(line, "[NEW]") || strings.Contains(line, "[MODIFY]") || strings.Contains(line, "[DELETE]")

			for _, match := range matches {
				label := strings.TrimSpace(match[1])
				urlStr := strings.TrimSpace(match[2])

				if isActionHeading || strings.HasPrefix(urlStr, "file://") {
					if err := validateFileLink(line, label, urlStr, isCompleted, workspaceRoot); err != nil {
						errs = append(errs, fmt.Errorf("line %d: %w", lineNum, err))
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		errs = append(errs, err)
	}
	return errs
}

func validatePlanFile(filePath string, workspaceRoot string) []error {
	var errs []error

	hasTopLevelHeader, hasProposedChanges, hasVerificationPlan, isCompleted, err := parsePlanMetadata(filePath)
	if err != nil {
		return []error{fmt.Errorf("failed to parse plan metadata: %w", err)}
	}

	if !hasTopLevelHeader {
		errs = append(errs, fmt.Errorf("missing top-level heading matching '# plan: Task ...'"))
	}
	if !hasProposedChanges {
		errs = append(errs, fmt.Errorf("missing heading '## Proposed Changes'"))
	}
	if !hasVerificationPlan {
		errs = append(errs, fmt.Errorf("missing heading '## Verification Plan'"))
	}

	linkErrs := validateFileLinks(filePath, isCompleted, workspaceRoot)
	errs = append(errs, linkErrs...)

	return errs
}

func main() {
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		fmt.Printf("Error: could not determine workspace root: %v\n", err)
		os.Exit(1)
	}

	templatePath := filepath.Join(workspaceRoot, "plans", "TEMPLATE.md")
	fmt.Printf("Reading template structure from %s...\n", templatePath)
	err = validateTemplate(templatePath)
	if err != nil {
		fmt.Printf("Error: template structure validation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Template structure is valid.")

	plansPattern := filepath.Join(workspaceRoot, "plans", "task_*.md")
	planFiles, err := filepath.Glob(plansPattern)
	if err != nil {
		fmt.Printf("Error: glob pattern failed: %v\n", err)
		os.Exit(1)
	}

	if len(planFiles) == 0 {
		fmt.Println("No task plans found to validate.")
		os.Exit(0)
	}

	fmt.Printf("Found %d implementation plan(s) to validate.\n", len(planFiles))
	hasErrors := false

	for _, planFile := range planFiles {
		relPath, _ := filepath.Rel(workspaceRoot, planFile)
		fmt.Printf("Validating plan: %s...\n", relPath)
		errs := validatePlanFile(planFile, workspaceRoot)
		if len(errs) > 0 {
			hasErrors = true
			fmt.Printf("FAIL: %s has validation errors:\n", relPath)
			for _, e := range errs {
				fmt.Printf("  - %v\n", e)
			}
		} else {
			fmt.Printf("PASS: %s is valid\n", relPath)
		}
	}

	if hasErrors {
		fmt.Println("\nPlan conformance verification failed.")
		os.Exit(1)
	}

	fmt.Println("\nAll implementation plans conform to structural templates.")
}
