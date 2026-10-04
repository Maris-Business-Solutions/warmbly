# Pipedrive app extensions

The Warmbly panel on Pipedrive person and deal pages, and its two modals, as
JSON schemas to paste into the Pipedrive Developer Hub (App extensions):

| File | Extension | API endpoint |
| --- | --- | --- |
| `panel.schema.json` | JSON panel, multiple objects, on Person and Deal details | `<backend>/api/v1/integrations/pipedrive/app/panel` |
| `enroll-modal.schema.json` | JSON modal "Add to Warmbly campaign" | `<backend>/api/v1/integrations/pipedrive/app/enroll` |
| `pause-modal.schema.json` | JSON modal "Pause in Warmbly" | `<backend>/api/v1/integrations/pipedrive/app/pause` |

Leave every JWT secret blank so Pipedrive signs with the app's client secret,
which is what the backend verifies. Add both modals as actions of the panel.
The full setup is in the docs: `docs/content/docs/development/configuration.mdx`
(Pipedrive app) and `docs/content/docs/guides/pipedrive.mdx`.
