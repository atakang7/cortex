GET /tasks returns everything at once, which no longer works for our bigger lists. Add filtering and pagination to it:

- `done=true|false` keeps only tasks in that state; `tag=<name>` keeps only tasks with that tag. Both can be combined.
- `limit` (default 20, allowed 1 to 100) and `cursor` page through the results in id order. The response becomes `{"tasks": [...], "next_cursor": "..."}`; passing next_cursor back as `cursor` gives the next page, and next_cursor is empty on the last page. Pages must not skip or repeat tasks when tasks are deleted between requests.
- Any invalid parameter is a 400 with the usual error body.

Update the API section of the README, and add tests for the new behaviour.
