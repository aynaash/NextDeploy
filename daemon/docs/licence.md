

### 🧱 Backend: CLI + Daemons (Go)

→ **License: Apache 2.0**

### 🎛️ Frontend: Dashboard UI (Next.js)

→ **License: Business Source License (BUSL-1.1)**

---

## 🔹 Step-by-Step Implementation

### 1. 🧱 Apache 2.0 for CLI + Daemons

Create a file in your Go backend repo:

`LICENSE`:

```text
                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION...

[Insert full Apache 2.0 text – available at: https://www.apache.org/licenses/LICENSE-2.0.txt]
```

Then in each Go source file, top-of-file comment:

```go
// Copyright 2025 NextDeploy
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
```

---

### 2. 🎛️ BUSL-1.1 for the Frontend (Next.js Dashboard)

In your dashboard repo:

#### `LICENSE`:

```text
Business Source License 1.1

Licensor: NextDeploy

Licensed Work: The NextDeploy dashboard, including but not limited to UI components, monitoring pages, log viewers, and admin panels.

The Licensed Work is provided under the terms of the Business Source License 1.1 (BUSL-1.1), a copy of which is available at https://mariadb.com/bsl11

Use Limitation: The Licensed Work may only be used for non-commercial purposes for up to three years from the date of release.

Change Date: [Three years from your release date]

Change License: Apache 2.0

After the Change Date, the Licensed Work will be made available under the Change License.
```

You can customize:

* **Use Limitation**: e.g., *"May not be used to offer a hosted platform similar to NextDeploy."*
* **Change Date**: Defaults to 3 years, change if you want
* **Commercial Use**: You may write a `COMMERCIAL.md` if you want to define how people can pay for usage.

---

## ✅ Add `LICENSE.md` or `README.md` Notices

In **both repos**, make this clear:

**Backend (Go CLI & Daemons):**

```md
## License

This repository is licensed under the Apache 2.0 License. See the [LICENSE](./LICENSE) file for more details.
```

**Frontend (Dashboard):**

```md
## License

This repository is licensed under the Business Source License 1.1 (BUSL-1.1). It is source-available, but not open-source. Commercial use is not permitted without a separate commercial license.

See the [LICENSE](./LICENSE) file for details.
```

---

## 📣 Final Touch: Dual-License Option

You can offer **commercial licensing** on the frontend later with a `COMMERCIAL.md` file:

```md
## Commercial Licensing

To use the NextDeploy dashboard in a commercial setting, or to integrate it into a paid product or hosting service, please contact us at [your email or website].

We offer flexible commercial licensing based on your use case.
```

---

## 🧨 Bottom Line

This is the best move.

* ✅ Apache 2.0 → drives adoption, builds trust, and keeps the dev community onboard.
* ✅ BUSL → keeps your moat protected from parasites and Vercel-wannabes.
* ✅ Dual licensing option → lets you monetize without locking the ecosystem.

If you want, I’ll generate the full boilerplate LICENSE files ready to drop into each repo. Just say the word.
