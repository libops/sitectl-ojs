# sitectl-ojs

`sitectl-ojs` is the LibOps sitectl plugin for Open Journal Systems.

It registers a first-class create definition for `https://github.com/libops/ojs` so the stack can be installed with:

```bash
sitectl create ojs
```

It also provides context-aware helpers:

- `sitectl ojs build`
- `sitectl ojs init`
- `sitectl ojs up`
- `sitectl ojs down`
- `sitectl ojs status`
- `sitectl ojs logs [SERVICE...]`
- `sitectl ojs rollout`

OJS-specific helpers:

- `sitectl ojs tool TOOL [args...]`
- `sitectl ojs pkp-tool TOOL [args...]`
- `sitectl ojs upgrade [args...]`
- `sitectl ojs import-export [args...]`
- `sitectl ojs rebuild-search-index [args...]`
- `sitectl ojs scheduled-tasks [args...]`
- `sitectl ojs jobs [args...]`
- `sitectl ojs plugins [args...]`
