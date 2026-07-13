const fs = require('fs');
const { execSync } = require('child_process');

try {
  const prNumber = process.env.PR_NUMBER;
  const baseRef = process.env.BASE_REF;
  const prHeadSha = process.env.PR_HEAD_SHA;

  if (!prNumber || !baseRef || !prHeadSha) {
    console.error("Missing environment variables: PR_NUMBER, BASE_REF, or PR_HEAD_SHA.");
    process.exit(1);
  }

  console.log(`Analyzing changes in PR #${prNumber} (HEAD: ${prHeadSha}) compared to base ref '${baseRef}'...`);

  // 1. Fetch the base branch and PR head SHA from the remote repository to ensure we can diff against them
  try {
    console.log(`Fetching origin/${baseRef}...`);
    execSync(`git fetch origin ${baseRef} --depth=1`, { stdio: 'inherit' });
  } catch (err) {
    console.log(`[WARNING] Failed to fetch origin/${baseRef} with depth=1, attempting full fetch...`);
    execSync(`git fetch origin ${baseRef}`, { stdio: 'inherit' });
  }

  try {
    console.log(`Fetching PR head SHA ${prHeadSha}...`);
    execSync(`git fetch origin ${prHeadSha} --depth=1`, { stdio: 'inherit' });
  } catch (err) {
    console.log(`[WARNING] Failed to fetch PR head ${prHeadSha} with depth=1, attempting full fetch...`);
    execSync(`git fetch origin ${prHeadSha}`, { stdio: 'inherit' });
  }

  // 2. Find modified files in this PR
  const diffOutput = execSync(`git diff --name-only origin/${baseRef}...${prHeadSha}`, { encoding: 'utf8' });
  const modifiedFiles = diffOutput.split('\n').map(f => f.trim()).filter(Boolean);
  console.log("Modified files detected:\n", modifiedFiles.map(f => ` - ${f}`).join('\n'));

  // 3. Scan modified files for plans and extract active Issue IDs
  const issueIds = new Set();
  const planRegex = /^plans\/.*\.md$/;
  const issueRegex = /Issue\s+#(\d+)/gi;

  for (const file of modifiedFiles) {
    if (planRegex.test(file)) {
      try {
        console.log(`Reading content of ${file} at SHA ${prHeadSha} using git show...`);
        const content = execSync(`git show ${prHeadSha}:${file}`, { encoding: 'utf8' });
        for (const match of content.matchAll(issueRegex)) {
          issueIds.add(match[1]);
          console.log(`Found Issue ID #${match[1]} inside plan file: ${file}`);
        }
      } catch (err) {
        console.error(`[WARNING] Failed to read file ${file} at SHA ${prHeadSha}:`, err.message);
      }
    }
  }

  if (issueIds.size === 0) {
    console.log("No task plan files with linked Issue IDs were modified in this PR. Exiting.");
    process.exit(0);
  }

  // 4. Retrieve the current PR body text using GitHub CLI
  console.log("Retrieving current PR description...");
  const prBody = execSync(`gh pr view ${prNumber} --json body --jq .body`, { encoding: 'utf8' }).trim();
  console.log("Current PR description:\n----------------------\n" + prBody + "\n----------------------");

  // 5. Determine which Issue IDs are not already referenced in the PR description
  const missingRefs = [];
  for (const id of issueIds) {
    // Check for standard GitHub closing keywords followed by the issue reference (e.g. Closes #12, Fixes #12, Resolves #12)
    const closesPattern = new RegExp(`(?:closes|resolves|fixes)\\s+#${id}\\b`, 'i');
    if (!closesPattern.test(prBody)) {
      missingRefs.push(id);
    }
  }

  if (missingRefs.length === 0) {
    console.log("All matching issue references are already present in the PR description. No update needed.");
    process.exit(0);
  }

  console.log("Missing issue references to link:", missingRefs.map(id => `#${id}`).join(', '));

  // 6. Build the new PR body and save it to a temporary file for update
  let newBody = prBody;
  if (newBody.length > 0 && !newBody.endsWith('\n')) {
    newBody += '\n';
  }

  const marker = '<!-- Auto-linked via CI plan checker -->';
  if (!newBody.includes(marker)) {
    newBody += `\n${marker}`;
  }
  for (const id of missingRefs) {
    newBody += `\nCloses #${id}`;
  }

  console.log("Updating PR description body...");
  const tempFile = 'temp_pr_body.txt';
  fs.writeFileSync(tempFile, newBody, 'utf8');

  execSync(`gh pr edit ${prNumber} --body-file ${tempFile}`, { stdio: 'inherit' });
  fs.unlinkSync(tempFile);

  console.log("[SUCCESS] Successfully updated the PR description with closing references.");
} catch (error) {
  console.error("Error executing link_task_issue script:", error);
  process.exit(1);
}
