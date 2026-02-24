# 🔄 Retrospectives

Retrospectives in **AgileLedger** are a deeper extension of the daily standups. Once a cycle or Sprint finishes, teams can launch a Retro ritual, looking back to improve processes while linking back to data stored throughout the daily sprints.

## Retro Workflows

- **Custom Configurations**: Define specific questions tailored for sprint retros (e.g. "What went well?", "What could we improve?", "Action Items"). Parameters setup the required workflow reviews.
- **Data Integration**: A core feature of AgileLedger retrospectives is data connectivity. When completing a retro form in the Web UI, users can tap into and reference their daily standups. For instance, specific accomplishments outlined in past "What I've done yesterday" entries can be seamlessly moved over to populate their "My Top Achievements" for the Sprint.

## Metrics & Dashboards

As data consolidates into `ingitdb` collections, metrics can be beautifully populated onto interactive dashboards outlining historical Sprint metrics:

- **Interaction Health**: Summaries of how many 'thumbs-up' each individual gave out, and received, encouraging positive team habits.
- **Participation**: Completion rates for daily standups. Spot gaps in communication simply by looking at automated charts.
- **Highlights**: Aggregated, team-wide top achievements based on user tags and submissions.

Metrics are built without a backend server—all calculated quickly post-PR-merge using `ingitdb` materialized views.
