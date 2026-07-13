# GitHub Integration

annet-oil can integrate with GitHub for linking deployments to pull requests and issues.

## Configuration

Add the `integrations.github` section to your `config.yaml`:

```yaml
integrations:
  github:
    enabled: true
    token: "${GITHUB_TOKEN}"
```

## Options

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | bool | Yes | Enable/disable GitHub integration |
| `token` | string | Yes | GitHub personal access token or GitHub App token |

## Getting a Token

### Personal Access Token (Classic)

1. Go to https://github.com/settings/tokens
2. Click "Generate new token (classic)"
3. Select scopes:
   - `repo` - Full control of private repositories
   - Or `public_repo` - For public repositories only
4. Generate and copy the token

### Fine-grained Personal Access Token (Recommended)

1. Go to https://github.com/settings/tokens?type=beta
2. Click "Generate new token"
3. Select repository access (specific repos or all)
4. Set permissions:
   - Issues: Read and write
   - Pull requests: Read and write
5. Generate and copy the token

## Environment Variables

Store the token securely:

```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxx"
```

Reference in config:
```yaml
integrations:
  github:
    token: "${GITHUB_TOKEN}"
```

## Usage

### Link Deployment to PR

After a successful deployment, add a comment to the related PR:

```go
import "annet-oil/pkg/github"

client := github.NewClient(github.Config{
    Token: os.Getenv("GITHUB_TOKEN"),
})

client.AddDeployComment(ctx, "org", "repo", 42, github.DeployInfo{
    User:   "alice",
    Hosts:  []string{"router-1", "router-2"},
    Result: "success",
    Time:   time.Now(),
})
```

### Link to Issue

```go
client.AddIssueComment(ctx, "org", "repo", 123, 
    "Configuration deployed successfully")
```

### Get PR Details

```go
pr, err := client.GetPullRequest(ctx, "org", "repo", 42)
fmt.Printf("PR #%d: %s (%s)\n", pr.Number, pr.Title, pr.State)
```

## API Reference

### Client Methods

| Method | Description |
|--------|-------------|
| `AddPRComment(ctx, owner, repo, prNum, body)` | Add comment to PR |
| `AddIssueComment(ctx, owner, repo, issueNum, body)` | Add comment to issue |
| `AddDeployComment(ctx, owner, repo, prNum, info)` | Add formatted deploy comment |
| `GetPullRequest(ctx, owner, repo, prNum)` | Get PR details |
| `GetIssue(ctx, owner, repo, issueNum)` | Get issue details |

### Deploy Comment Format

When using `AddDeployComment`, the comment is formatted as:

```markdown
### Deployment Executed

**User:** alice
**Hosts:** [router-1, router-2]
**Result:** success
**Time:** 2024-01-15T10:30:00Z
```

## GitHub Enterprise

For GitHub Enterprise Server, set a custom base URL:

```go
client := github.NewClient(github.Config{
    Token:   os.Getenv("GITHUB_TOKEN"),
    BaseURL: "https://github.company.com/api/v3",
})
```

## Future Enhancements

Planned features:
- MCP tools for GitHub operations (`annet_github_comment`, etc.)
- Automatic PR comments on deploy
- Status checks integration
- Deployment environments

## Troubleshooting

### 401 Bad Credentials

- Token is invalid or expired
- Token doesn't have required scopes

### 404 Not Found

- Repository doesn't exist
- Token doesn't have access to the repository
- PR/Issue number is wrong

### 403 Forbidden

- Token lacks required permissions
- Rate limit exceeded (check `X-RateLimit-Remaining` header)
