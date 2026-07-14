const { execFileSync } = require('child_process');

// execFileSync is used with argument arrays (no shell spawn) to prevent shell command injection.
// All arguments (PR number, base ref, head SHA, file paths) are strictly validated/regex-checked before execution.
function runGit(args) {
  return execFileSync('git', args, { encoding: 'utf8' }).trim();
}

function runGitInherit(args) {
  execFileSync('git', args, { stdio: 'inherit' });
}

function runGh(args) {
  return execFileSync('gh', args, { encoding: 'utf8' }).trim();
}

try {
  const prNumber = process.env.PR_NUMBER;
  const baseRef = process.env.BASE_REF;
  const prHeadSha = process.env.PR_HEAD_SHA;

  if (!prNumber || !baseRef || !prHeadSha) {
    console.error("Missing environment variables: PR_NUMBER, BASE_REF, or PR_HEAD_SHA.");
    process.exit(1);
  }

  // Validate PR number and SHA to protect against injection
  if (!/^\d+$/.test(prNumber)) {
    console.error(`Invalid PR_NUMBER: ${prNumber}`);
    process.exit(1);
  }
  if (!/^[a-fA-F0-9]{40}$/.test(prHeadSha)) {
    console.error(`Invalid PR_HEAD_SHA: ${prHeadSha}`);
    process.exit(1);
  }
  // Validate baseRef is a clean branch name (alphanumeric, slash, hyphen, underscore)
  // Rejects branch names starting with '-' to prevent option injection in git fetch.
  if (!/^[a-zA-Z0-9_\/][a-zA-Z0-9_\-\/]*$/.test(baseRef)) {
    console.error(`Invalid BASE_REF: ${baseRef}`);
    process.exit(1);
  }

  console.log(`Analyzing changes in PR #${prNumber} (HEAD: ${prHeadSha}) compared to base ref '${baseRef}'...`);

  // 1. Fetch the PR head SHA from the remote repository to ensure we can diff against it
  try {
    console.log(`Fetching PR head pull ref for PR #${prNumber}...`);
    runGitInherit(['fetch', 'origin', `pull/${prNumber}/head`]);
  } catch (err) {
    console.error(`[ERROR] Failed to fetch PR head pull ref:`, err.message);
    process.exit(1);
  }

  // 2. Find modified files in this PR
  const diffOutput = runGit(['diff', '--name-only', 'HEAD...FETCH_HEAD']);
  const modifiedFiles = diffOutput.split('\n').map(f => f.trim()).filter(Boolean);
  console.log("Modified files detected:\n", modifiedFiles.map(f => ` - ${f}`).join('\n'));

  // 3. Scan modified files for plans and extract active Issue IDs
  const issueIds = new Set();
  const planRegex = /^plans\/[a-zA-Z0-9_\-]+\/[a-zA-Z0-9_\-]+\.md$/;
  const issueRegex = /Issue\s+#(\d+)/gi;

  for (const file of modifiedFiles) {
    if (planRegex.test(file)) {
      try {
        console.log(`Reading content of ${file} at FETCH_HEAD using git show...`);
        const content = runGit(['show', `FETCH_HEAD:${file}`]);
        for (const match of content.matchAll(issueRegex)) {
          issueIds.add(match[1]);
          console.log(`Found Issue ID #${match[1]} inside plan file: ${file}`);
        }
      } catch (err) {
        console.error(`[WARNING] Failed to read file ${file} at FETCH_HEAD:`, err.message);
      }
    } else {
      if (file.startsWith('plans/')) {
        console.log(`[WARNING] Skipping file ${file} - does not match strict plan naming convention.`);
      }
    }
  }

  if (issueIds.size === 0) {
    console.log("No task plan files with linked Issue IDs were modified in this PR. Exiting.");
    process.exit(0);
  }

  // 4. Retrieve the current PR body text using GitHub CLI
  console.log("Retrieving current PR description...");
  const prBody = runGh(['pr', 'view', prNumber, '--json', 'body', '--jq', '.body']).trim();
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

  // 6. Build the new PR body
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
  runGh(['pr', 'edit', prNumber, '--body', newBody]);

  console.log("[SUCCESS] Successfully updated the PR description with closing references.");
} catch (error) {
  console.error("Error executing link_task_issue script:", error.message || error);
  process.exit(1);
}
