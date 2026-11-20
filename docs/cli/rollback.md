# `nextdeploy rollback`

Instantly revert the live workload to an earlier deployment version without rebuilding.

---

## Usage

```bash
nextdeploy rollback [flags]
```

### Flags

| Flag | Type | Description |
| :--- | :--- | :--- |
| `--steps` | integer | Number of deployments to walk back (default: `1`). |
| `--to` | string | Specific git commit hash or deployment release identifier to restore. |

---

## How Rollback Operates

- **On VPS**: Re-points the `current` symlink to an older release directory in `/var/www/apps/<app>/releases/`. Because `shared_static` accumulates all historical chunks, the rolled-back app has all necessary static assets available instantly without missing chunk 404s.
- **On Cloudflare Workers**: Reverts Worker deployment active version to the designated prior release and preserves matching R2 fingerprint prefixes.
- **On AWS Lambda**: Updates the CloudFront distribution origin or Lambda function alias to target the prior Lambda version.

### Example

```bash
# Roll back to the immediately previous release
nextdeploy rollback

# Roll back two deployments
nextdeploy rollback --steps 2

# Roll back to a specific commit
nextdeploy rollback --to e4c9a81
```
