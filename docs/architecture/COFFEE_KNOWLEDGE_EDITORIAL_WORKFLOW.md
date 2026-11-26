# Coffee Knowledge Editorial Workflow & Permission Matrix (Debt Closed)

## 1. Lifecycle States
`draft` → `submitted` → `under_review` → `approved` → `published` → `superseded` / `archived`.

## 2. Editorial Permission Matrix
Editorial roles are **platform-level** and **strictly independent** from `organization` roles (`owner`/`admin` grant zero editorial privileges).

| Action | `knowledge_editor` | `knowledge_reviewer` | `super_admin` | `reader` | Audited? |
|---|:---:|:---:|:---:|:---:|:---:|
| Create / Edit `draft` | Yes (own/team) | No | Yes | No | No |
| Submit for review | Yes | No | Yes | No | Yes |
| Assign reviewer | No | Yes | Yes | No | Yes |
| Approve / Reject | No | Yes (`!= author`) | Yes (`!= author`) | No | Yes |
| Publish / Supersede | No | Yes (`!= author`) | Yes | No | Yes |
| Archive | No | Yes | Yes | No | Yes |
| Correct published content (new revision) | Yes (creates draft) | Yes (approves) | Yes | No | Yes |
| Manage provenance & licensing | Yes (draft) | Yes (verify) | Yes | No | Yes |
| Emergency unpublish | No | No | Yes | No | Yes |

## 3. Separation of Duties
- **Self-approval is always prohibited**: `reviewer_id != author_id` is enforced in domain logic.
- **Approver vs. Publisher**: A `knowledge_reviewer` who approved an item may publish it (provided `reviewer_id != author_id`). A `knowledge_editor` can never approve or publish their own content.
