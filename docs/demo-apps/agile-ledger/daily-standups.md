# 📅 Daily Standups (Scrum)

Daily Standups in **AgileLedger** are entirely powered by the Git repository and validated by **ingitdb**. Members can submit answers to configured questions, interact with teammates' updates, and view daily summaries through materialized views.

## Overview & Workflow

1. **Configurable Questions**: A repository admin configures a list of mandatory and optional questions (e.g., "What did I accomplish yesterday?", "What are my priorities today?", "Any blockers?").
2. **Date-Based Organization**: Every day has its own dedicated directory/collection where users' answers are individually stored.
3. **Data Submission**: When a team member submits their standup via the SPA Web UI, it silently uses the GitHub API to create a personal branch, write the relevant JSON/YAML data files, and opens a Pull Request (PR).
4. **Validation & Merge**: GitHub Actions run the `ingitdb` CLI to perform incremental validation, strictly checking schema validity and data structure. Once passed, the PR auto-merges into the `main` branch.
5. **Materialized Views**: IngitDB then generates cached views, resulting in instant rendering of the data in the Web UI, aggregating standups per user, per day, and per question.

## Answer Management & Interactions

- **Editing & Deleting**: Users retain complete control over their entries. They can edit or delete their own answers directly, which fires another PR/Merge cycle in the background.
- **Reactions**: Team members can explore their colleagues' standups and leave an interactive 'thumbs-up' (👍) to entire statuses or specific individual answers. Notification triggers optionally inform the receiver via Slack or Telegram bots that they were given a thumbs-up.

## Data Schema

The directory structure and relationships for a daily standup setup are completely file-based. Using ingitdb conventions, AgileLedger organizes the hierarchy as follows:

- **Team Mates**:
  `/teams/{team}/mates/{github_user_name}.json`
  Defines individual team members and their specific data or roles.
  _Definition (in `teams/mates/.definition.yaml`):_
  ```yaml
  titles:
    en: Team Mates
  record_file:
    name: "{key}.json"
    type: "map[string]any"
    format: json
  ```
- **Daily Questions**:
  `/teams/{team}/daily-questions/daily-questions.json`
  Stores the template of mandatory and optional questions (e.g., `what-i-did-yesterday`, `what-i-will-do-today`) to be used in standups.
  _Definition (in `teams/daily-questions/.definition.yaml`):_

  ```yaml
  titles:
    en: Daily Questions
  record_file:
    name: "daily-questions.json"
    type: "map[string]any"
    format: json
  ```

- **Dates**:
  `/teams/{team}/dates/{yyyy-mm-dd}/{yyyy-mm-dd}.json`
  Holds top-level metadata about a specific date (e.g., if a date was a public holiday, sprint boundary, etc.).
  _Definition (in `teams/dates/.definition.yaml`):_

  ```yaml
  titles:
    en: Dates
  record_file:
    name: "{key}/{key}.json"
    type: "map[string]any"
    format: json
  ```

- **User Answers**:
  `/teams/{team}/dates/{yyyy-mm-dd}/mates/{github_user_name}/questions/{question_slug}/answers/{answer_timestamp}.json`
  Individual answers are stored granularly by user, question slug, and the exact timestamp of submission. This structure avoids concurrent write conflicts, making it ideal for the underlying git repository.
  _Definition (in `teams/dates/mates/questions/answers/.definition.yaml`):_
  ```yaml
  titles:
    en: Answers
  record_file:
    name: "{key}.json"
    type: "map[string]any"
    format: json
  ```
