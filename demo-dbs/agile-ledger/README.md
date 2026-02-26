# AgileLedger

> **Note**: This plan describes and relies on features of **ingitdb** that may not be implemented yet. The main purpose of this application is to help drive the development of ingitdb by serving as a comprehensive real-world use case and to showcase its various features.

AgileLedger is an open-source, **free to use** template app for agile teams that uses **ingitdb** as its main database, hosted directly in a Git repository. It is designed to facilitate agile rituals, specifically **Daily Standups** and **Retrospectives**, through a statically hosted GitHub Pages web UI without needing a traditional backend.

With AgileLedger, **users and teams completely own and control their data**. Storage and privacy are managed entirely through standard GitHub permissions.

## Overview

AgileLedger provides a completely serverless structure leveraging GitHub and ingitdb. By forking the AgileLedger template repo, a team immediately gets a pre-configured ingitdb setup.

- **Frontend**: A statically hosted Single Page Application (SPA), implemented in **Ionic / Angular**, that acts as the UI.
- **Backend/Data Layer**: GitHub API + ingitdb CLI (run via GitHub Actions). Data is stored as JSON/YAML files in the repo.
- **Data Flow**: When a user submits an update, the web UI uses the GitHub API to create a personal branch, add/modify the relevant files, and open a Pull Request (PR).
- **Automation**: IngitDB CLI runs via GitHub Actions to automatically validate both the schema and the data via incremental checks. Once validated, the PR is automatically merged into the main branch, triggering a process to update the materialized views that power summary pages.

## Architecture & Permissions

1. **GitHub Repository Store**: Branches hold pending changes; `main` holds the source of truth.
2. **ingitdb Engine**: Validates data schemas (mandatory/optional questions, date formats) and data integrity (e.g., verifying authorship) using incremental checks on PR validation. It then generates materialized views for quick reads.
3. **AgileLedger Web UI**: Statically deployed SPA using Ionic/Angular. It authenticates users via GitHub OAuth, allowing them to read data and write data back by utilizing the GitHub Repositories API.
4. **Teams & Roles**:
   - A single repository can host **multiple teams**.
   - You can specifically define team members and their roles.
   - **Only team members** are authorized to submit data to a team's ledger.
   - **Team spectators** can be granted read-only access to view the team's data.
5. **Privacy & Granular Permissions**:
   - Base privacy is managed through standard GitHub repository permissions.
   - If stricter, more granular permissions are required, a decentralized workflow can be utilized: Users fork the main team repository into their own private accounts, make changes, and the UI automatically submits a PR from their forked repo back to the team repository.

## Core Features

### ⚡️ Triggers & Automation

- **Proactive Reminders**: `ingitdb` triggers can be set up to automatically remind team members to submit their answers prior to a scheduled standup or retrospective.
- **Real-Time Notifications**: Triggers can be utilized to send webhook notifications (e.g., via a Telegram bot, Slack, etc.) for various events, such as when a user submits their standup answers, or when a user's answer receives a 'thumbs-up' (👍) from a teammate.

### 📅 Daily Standups (Scrum)

Daily standups allow team members to submit updates on their progress for a specific date, configured via customized mandatory and optional questions. It stores answers in collections powered by ingitdb validations, and automatically merges changes through personal PRs.

[**📖 Read more about Daily Standups**](./daily-standups.md)

### 🔄 Retrospectives

Retrospectives help agile teams reflect on past sprints by enabling custom review workflows. Users can pull data directly from their daily standups (like tagging an accomplishment) and convert them into top achievements, tracking team performance metrics aggregated by ingitdb.

[**📖 Read more about Retrospectives**](./retrospectives.md)

## Roadmap & Development Plan

### Phase 1: Foundational Setup (Template Repository)

- [ ] Initialize the `agile-ledger` base template repository.
- [ ] Outline the ingitdb collections structure (`teams`, `users`, `questions`, `daily_standups`, `retrospectives`, `reactions`).
- [ ] Create GitHub Action workflows for ingest validation (using incremental checks) and automatic PR merging.
- [ ] Define IngitDB validation rules (ensuring only team members can edit entries, validating roles, etc.).

### Phase 2: Web Application scaffolding

- [ ] Set up the static Web UI skeleton using Ionic / Angular.
- [ ] Implement GitHub OAuth for secure user login.
- [ ] Build GitHub API bindings for reading repo contents, forking repos, creating branches, committing files, and creating PRs.

### Phase 3: Teams & Daily Standup Workflows

- [ ] Implement multi-team configurations, role assignments, and spectator controls.
- [ ] Develop UI for answering Daily Standup questions.
- [ ] Develop UI for reading Standup summaries.
- [ ] Implement reaction system (thumbs-up) on standup answers.
- [ ] Set up materialized views via ingitdb to serve aggregated data (per user, per day) to the UI.
- [ ] Implement triggers and notifications for daily standup reminders and interactions.

### Phase 4: Retrospective Workflows

- [ ] Develop UI for defining and starting a Retro ritual.
- [ ] Build data-fetching mechanism allowing users to refer back to their Daily Standup logs.
- [ ] Implement metrics compilation (thumbs-up counters, sprint highlights) leveraging ingitdb.
- [ ] Create interactive retrospective boards driven by GitHub PRs in the background.
