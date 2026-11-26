# Bot catalog review

Reviewed the 32 existing bots against their Go code, setup instructions, and repository workflows. The review considered how often the job comes up, how much manual work it saves, and what can go wrong when it runs unattended.

## Changes

- Retired **merge-queue-assistant**. It suggests an order with labels and comments. Teams that need merge order enforced and checks rerun can use GitHub’s merge queue where available. [GitHub merge queue](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue)
- Retired **dependency-update-grouper**. It labels pull requests by parsing their titles. Dependabot’s groups put updates together in a single pull request. [Dependabot grouping](https://docs.github.com/en/code-security/tutorials/secure-your-dependencies/optimizing-pr-creation-version-updates)
- Retired **issue-link-checker**. It makes server-side requests to URLs supplied by issue authors. Its broken-link results can also be misleading when a site refuses automated requests. The documentation link checker remains available.
- Added **[default-branch-ci-watchdog](../bots/default-branch-ci-watchdog)**. It opens or updates one issue when a selected workflow on the default branch fails, links to the failed run, and closes the issue after a successful run.

Existing installs of retired bots can have their workflow or webhook disabled. This change leaves historical release artifacts and existing GitHub issues, labels, and comments as they are.

## What the existing bots do

| Bot | Review |
| --- | --- |
| pr-size-labeler | Keep. The size label gives reviewers useful context before opening the diff. |
| first-touch-welcomer | Keep. Maintainers can point newcomers to project guidelines automatically. |
| stale-bot | Keep for large backlogs. Review its exemptions and dry-run output before enabling automatic closure. |
| branch-sweeper | Keep for its branch safety checks. GitHub’s delete-on-merge setting covers simpler repositories. |
| conventional-pr-check | Keep. A required title check helps teams that produce changelogs from squash commits. |
| reviewer-assigner | Keep. Path-based routing helps projects with different owners. Its custom configuration fallback needs separate review. |
| duplicate-issue-hunter | Keep. It suggests possible matches and leaves the closure decision to a maintainer. |
| todo-tracker | Keep for teams that explicitly track code TODOs as issues. It can produce a noisy backlog, and its diff-based tracking has limitations. |
| merge-conflict-detector | Keep. The bot tells an author when a base-branch change has made their pull request harder to merge. |
| auto-merger | Keep for compatibility. Its custom label path needs a separate review of how it verifies approvals and missing checks. |
| repo-digest | Keep. A weekly issue makes activity visible without opening several GitHub views. |
| pr-template-checker | Keep. It reminds contributors to fill in the headings reviewers rely on. |
| release-drafter | Keep. It prepares release notes throughout the release, when the reasons for changes are still fresh. |
| keyword-issue-labeler | Keep. Projects can control which terms send new issues into which queues. |
| ci-failure-commenter | Keep. A single updated pull request comment provides failure links and clears stale feedback after recovery. |
| pr-aging-bot | Keep for teams sorting a whole pull request backlog by age. Its work overlaps with review reminders, which focus on individual reviewers. |
| review-reminder | Keep. It targets people who have been asked to review, so reminders are actionable. |
| issue-triage-reminder | Keep. A visible flag helps maintainers find reports that still need attention. |
| changelog-checker | Keep for projects that maintain a changelog by hand. Its generated Actions permissions need separate review. |
| license-header-checker | Keep for repositories with a license-header policy. A source-level CI lint may suit stricter requirements. |
| pr-path-labeler | Keep. Area labels help maintainers route changes across large repositories. |
| contributor-milestones | Keep as opt-in community automation. It matters most where maintainers want milestone recognition. |
| breaking-change-guard | Keep for repositories that define paths needing migration review. The status is a prompt for human review. |
| release-reminder | Keep for projects that accumulate work between releases. A future weekly digest could include the same backlog signal. |
| docs-link-checker | Keep. One tracking issue gives maintainers a repair list for broken documentation links. |
| issue-link-checker | Retired. The destination and availability risks outweigh checking links from issue text. |
| issue-form-validator | Keep for freeform or legacy issue templates. GitHub Issue Forms can enforce required fields at submission on newer projects. |
| dependency-update-grouper | Retired. Use grouping in the dependency updater to create actual batches. |
| flaky-test-tracker | Keep where CI history can persist across runs. It detects inconsistent check results and needs durable state to do that reliably. |
| merge-queue-assistant | Retired. A label-based order cannot enforce fresh checks or merge order. |
| discussion-triage-bot | Keep. It gives maintainers one list of discussions with no replies. |
| project-board-sync | Keep for assignment-driven board transitions. Projects writes require suitable credentials beyond the default workflow token. |

The individual review was static. Live GitHub behavior of the retained bots and their existing workflows needs separate verification.
