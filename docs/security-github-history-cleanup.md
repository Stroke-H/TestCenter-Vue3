# GitHub History Cleanup Plan

This repository previously tracked runtime secrets and local data files.
The latest tree should stop tracking those files, but Git history can still contain old values.

## Immediate Actions

1. Rotate exposed credentials before relying on history cleanup:
   - DeepSeek API key
   - Claude API key, if still active
   - Feishu app secret
   - Feishu MCP token
   - Any captured JWT or `x_token`
2. Confirm the latest commit no longer tracks runtime data files:
   - `server/data/ai_config.json`
   - `server/data/feishu_config.json`
   - `drama_info.json`
   - `backup/legacy/*.jsonl`
   - `server/feishu/*.jsonl`
   - `k6-scripts/reports/*`

## Optional History Rewrite

Only do this after coordinating with anyone else using the repository, because it rewrites commit history.
`git filter-repo` is preferred. If it is unavailable, use `git filter-branch` as a local fallback and force-push the cleaned branch with lease protection.

```bash
git filter-repo \
  --path server/data/ai_config.json \
  --path server/data/feishu_config.json \
  --path drama_info.json \
  --path-glob 'backup/legacy/*.jsonl' \
  --path-glob 'server/feishu/*.jsonl' \
  --path-glob 'k6-scripts/reports/*' \
  --invert-paths
```

Fallback:

```bash
FILTER_BRANCH_SQUELCH_WARNING=1 git filter-branch --force --index-filter \
  'git rm -r --cached --ignore-unmatch server/data/ai_config.json server/data/feishu_config.json server/data/users.jsonl server/data/database_config.json drama_info.json k6-scripts/reports/drama_check_report.html server/feishu/test_phones.jsonl backup/legacy/acceptance_reports.jsonl backup/legacy/projects.jsonl backup/legacy/pw_cases.jsonl backup/legacy/pw_keywords.jsonl backup/legacy/pw_suites.jsonl backup/legacy/test_phones.jsonl backup/legacy/testcase_history.jsonl backup/legacy/users.jsonl' \
  --prune-empty --tag-name-filter cat -- main
```

Then force-push the cleaned branch:

```bash
git push origin main --force-with-lease
```

After a force push, collaborators should re-clone or carefully reset their local branches to the new history.
