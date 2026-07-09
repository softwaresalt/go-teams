---
description: "Backlog tool integration instructions — teaches agents how to interact with the installed backlog management tool using abstracted operations"
applyTo: '**'
---

# Backlog Integration Instructions

This workspace uses **backlogit** for structured backlog management. All agents MUST use the backlog tool for task tracking rather than creating ad-hoc markdown files or static task lists.

## Tool Configuration

| Setting | Value |
|---------|-------|
| Tool | backlogit |
| Directory | `.backlogit/` |
| Access | both (MCP + CLI) |
| Registry | `.autoharness/backlog-registry.yaml` |
| Project config | `.backlogit/config.yaml` |

## Operation Reference

Use these operations for all backlog interactions. The operation names are abstract — the actual tool names and parameters are mapped through the backlog registry.

### Core Operations (All Tools)

| Operation | MCP Tool | CLI Command | Purpose |
|-----------|----------|-------------|---------|
| Create task | `backlogit_create_item` | `backlogit add --type {artifact_type} --title {title}` | Create a new task/artifact |
| List tasks | `backlogit_list_items` | `backlogit list` | List tasks with filters |
| Get task | `backlogit_get_item` | `backlogit get {id}` | Retrieve task details |
| Update task | `backlogit_update_item` | `backlogit update {id}` | Modify task fields |
| Move task | `backlogit_move_item` | `backlogit move {id} --status {status}` | Change task status |
| Search | `backlogit_search_items` | `backlogit search {query}` | Full-text search |
| Complete | `backlogit_move_item` | `backlogit move {id} --status done` | Mark task complete |

### Status Values

| Abstract Status | Tool-Specific Value |
|----------------|---------------------|
| Queued | `queued` |
| Active | `active` |
| Done | `done` |
| Blocked | `blocked` |

> Note: this project's `.backlogit/config.yaml` defines a wider status
> enum (`queued`, `active`, `blocked`, `review`, `done`, `accepted`,
> `rejected`, `archived`). The four abstract statuses above map to the
> registry defaults; `review`, `accepted`, `rejected`, and `archived` are
> additional project-specific states agents may also encounter.

### Extended Operations (Tool-Dependent)

| Feature | MCP Tool(s) | Purpose |
|---|---|---|
| SQL query | `backlogit_query_sql` | Token-efficient targeted lookup instead of scanning files |
| Sync index | `backlogit_sync_index` | Refresh the index after out-of-band edits |
| Comments | `backlogit_append_comment` | Traceability notes on an item |
| Telemetry | `backlogit_log_telemetry`, `backlogit_telemetry_harvest` | Structured event logging and harvesting |
| Memory | `backlogit_save_memory` | Session continuity summaries |
| Checkpoints | `backlogit_create_checkpoint`, `backlogit_list_checkpoints`, `backlogit_get_checkpoint`, `backlogit_resolve_checkpoint` | Durable state snapshots across sessions |
| Hook events | `backlogit_poll_hook_events`, `backlogit_ack_hook_events` | Event-driven coordination |
| Queue | `backlogit_get_queue` | Ready-work selection with filters/grouping |
| Dependencies | `backlogit_add_dependency`, `backlogit_remove_dependency`, `backlogit_get_dependencies` | Sequencing and blocking relationships |
| Commit tracking | `backlogit_track_commit` | Associate commits with task IDs |
| Shipments | `backlogit_create_shipment`, `backlogit_get_shipment`, `backlogit_list_shipments`, `backlogit_claim_shipment`, `backlogit_ship_shipment`, `backlogit_add_to_shipment`, `backlogit_return_blocked` | Release-unit assembly and closure |
| Stash | `backlogit_stash`, `backlogit_stash_edit`, `backlogit_stash_get`, `backlogit_stash_remove`, `backlogit_stash_archive`, `backlogit_fetch_stash`, `backlogit_harvest_stash` | Parked ideas awaiting triage/promotion |
| Deliberation | `backlogit_deliberate` | Structured decision capture (problem frame, options, chosen direction) |
| Semantic links | `backlogit_add_link`, `backlogit_remove_link`, `backlogit_get_links` | Non-hierarchical relationships between items |
| Lifecycle hygiene | `backlogit_archive_item`, `backlogit_adopt_item`, `backlogit_doctor`, `backlogit_cleanup_checkpoints` | Archival, re-parenting, orphan/duplicate detection |
| Discovery | `backlogit_get_metadata_catalog`, `backlogit_list_types`, `backlogit_list_templates`, `backlogit_get_wit_metadata`, `backlogit_get_version`, `backlogit_export_command_map` | Type/field/section metadata and cached command reference |

## Agent Workflow Patterns

### Creating a Task

```text
Call backlogit_create_item with:
  title: "Task title"
  artifact_type: "task"
  status: "queued"
  description: "Task description"
  parent_id: "parent-task-id"  (if applicable)
  labels: "label1,label2"      (if applicable)
```

### Claiming a Task (Status → Active)

```text
Call backlogit_move_item with:
  id: "task-id"
  status: "active"
```

### Completing a Task

```text
Call backlogit_move_item with:
  id: "task-id"
  status: "done"
```

### Listing Ready Tasks

```text
Call backlogit_list_items with:
  status: "queued"
```

### Adding a Label

```text
Call backlogit_update_item with:
  id: "task-id"
  labels: "existing-label,harness-ready"
```

## Advanced Patterns When Supported

If the registry advertises advanced features, prefer them over ad hoc workarounds:

* **Token-efficient lookup** — use `backlogit_query_sql` (`features.sql_query` is true)
* **Ready-work selection** — use `backlogit_get_queue` (`features.queue` is true)
* **Dependency reasoning** — use dependency operations (`features.dependencies` is true)
* **Agent continuity** — use memory and checkpoint operations (`features.memory` and `features.checkpoints` are true)
* **Traceability** — use comment and commit-tracking operations (`features.comments` and `features.commit_tracking` are true)
* **Index freshness** — use `backlogit_sync_index` when the workspace was edited outside normal mutation tools

This project's registry reports `sql_query`, `telemetry`, `memory`,
`checkpoints`, `comments`, `dependencies`, `queue`, `commit_tracking`,
`shipments`, `stash`, `semantic_links`, `deliberation`, `discovery`, and
`lifecycle_hygiene` all enabled — prefer these over manual workarounds.
`milestones`, `documents`, `definition_of_done`, and `workflow_guides`
are NOT enabled for this project.

For type-specific field, section, and hierarchy details, follow the
tool-specific overlay instructions in
`.github/instructions/backlogit.instructions.md`,
`.github/instructions/backlogit-sql-schema.instructions.md`, and
`.github/instructions/backlogit-yaml-header-tooling.instructions.md`
in addition to this generic guide.

## Rules

1. **Always use the backlog tool** for task management. Do not create markdown task files outside the `.backlogit/` directory.
2. **Use abstract status values** mapped through the registry, not hardcoded strings.
3. **Check the registry** (`.autoharness/backlog-registry.yaml`) for the exact field names and operation parameters when unsure.
4. **Prefer MCP tools** over CLI when both are available — MCP returns structured JSON, CLI returns human-readable text.
5. **Feature gating**: Before calling an extended operation, verify the feature is supported by checking the `features` section in the registry.

Generated by autoharness | Template: backlog-integration.instructions.md.tmpl
