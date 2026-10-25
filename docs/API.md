# HTTP API

The Go runtime serves this API on the loopback interface (inside the Android
app) or wherever `cmd/career107` listens. All bodies are JSON. Domain refusals
are **not** HTTP errors: they come back with status 200 in `rejections`, in the
form `CODE: message`. HTTP 4xx/5xx is used only for malformed requests and
server faults.

## Read

| Method | Path | Returns |
| --- | --- | --- |
| GET | `/api/health` | `{status, runtime}` |
| GET | `/api/catalog` | Static vocabulary: instrument `items`, `capabilityContracts`, `contractVersions`, `invariantDescriptors`, `uncertaintyCodes`, `forbiddenScoreKeys`, statements, `mappingPolicy`, `subjectId`, `runtimeId`. |
| GET | `/api/session` | `SessionView` (below). |

`SessionView`:

| Field | Meaning |
| --- | --- |
| `projection` | Counters and contract versions (derived state). |
| `signals` | Captured observations (S0). |
| `representations` | 1:3 representations (S1). |
| `rendered` | Map representation id → reviewer rendering (A/E/C/U sections). |
| `pathwaySets` | Pathway sets, oldest first; the last is current. |
| `decisionSummaries` | Map set id → counts of user decisions. |
| `decisions`, `corrections`, `proposals` | Records. |
| `persisted` | Whether the session is saved to disk. |

## Actions

Every action returns `ActionResponse = {notice?, rejections[], view}`.

| Method | Path | Body |
| --- | --- | --- |
| POST | `/api/capture` | `{instrumentId, itemRef, response, declaredContext[], source, sessionRef?}` — `response` is `{kind:"option", optionId}`, `{kind:"options", optionIds}`, `{kind:"text", text}` or `{kind:"number", value}`. The server sets the subject and mints `inputId`. |
| POST | `/api/answer` | Same body as `/api/capture`. Captures the answer and decomposes it in one step (the app's question flow); both steps keep their own validation and authorization. |
| POST | `/api/decompose` | `{signalId}` |
| POST | `/api/map` | `{}` — maps the latest answer to each question. A changed answer is a new observation that supersedes the earlier one; the earlier one stays in `signals` and `representations`. |
| POST | `/api/review` | `{pathwayId, state, rationale?}` — `state` ∈ `interesting`, `not-a-fit`, `pursuing`, `deferred`; applies to the latest set; recorded as a decision by user 0. |
| POST | `/api/correct` | `{representationId, correctedSummary}` — adds a correction record; the representation is not edited. |
| POST | `/api/propose` | `{representationId}` — model-assisted follow-up questions, recorded as a proposal (stub provider). |
| POST | `/api/reset` | `{}` — discards the session and its saved journal. |
| POST | `/api/research/run` | `{}` → `{results[], reproducibility, report}` (markdown). |

Example:

```bash
curl -s localhost:8080/api/capture -d '{
  "instrumentId": "instrument.self-description",
  "itemRef": "itm-selfdesc-analysis-1",
  "response": {"kind": "option", "optionId": "opt.fully"},
  "declaredContext": [{"key": "setting", "value": "employed full-time"},
                      {"key": "careerStage", "value": "mid-career"}],
  "source": {"kind": "instrument-administration", "ref": "app://session/itm-selfdesc-analysis-1", "label": "app sitting"}
}' | jq '.notice, .rejections'
```

## Persistence

With a data directory, every action is appended to `session.json` together
with the clock readings it consumed. On start the journal is replayed, which
rebuilds exactly the same state — identities and digests included — because
every identity is derived from content and time. A journal that cannot be
replayed (for example after an incompatible upgrade) is renamed to
`session.json.unreadable-<time>` and a new session starts.
