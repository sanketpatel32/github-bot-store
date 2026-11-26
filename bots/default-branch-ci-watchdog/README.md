# Default branch CI watchdog

Find out when a change breaks the main branch. The bot checks the latest completed push run of one GitHub Actions workflow and opens an issue with a link to the failed run. New failures update that issue. A passing run closes it.

The bot ignores pull request runs, cancellations, and workflows with no completed push run. It leaves the alert open when it cannot confirm a passing run.

## Run in GitHub Actions

Choose **Alerts you when the main branch breaks** in the Bot Store, then copy the GitHub Actions setup. Set `CI_WORKFLOW` to the filename under `.github/workflows/` that runs on pushes to your default branch, such as `ci.yml`. Save the generated workflow on the default branch. GitHub runs the check hourly at minute 17. You can also run it from the Actions tab.

The generated workflow builds the bot from source because the current binary release predates it. Its `ref: main` follows this repository’s main branch. You can replace that ref with a reviewed commit SHA for a reproducible build. The workflow uses one concurrency group to serialize runs.

Permissions are Actions read and Issues write. The generated workflow also grants Contents read for checkout.

## Run on your server

```sh
go build -o default-branch-ci-watchdog ./bots/default-branch-ci-watchdog
export REPO=your-account/your-project CI_WORKFLOW=ci.yml
export GITHUB_TOKEN_FILE=/run/secrets/github_token
./default-branch-ci-watchdog
```

Use a token with Actions read and Issues write access to the configured repository. Run one instance per repository and workflow. Change the polling interval with `SWEEP_INTERVAL_HOURS`. The default is one hour, and the health endpoint listens on port 8133.
