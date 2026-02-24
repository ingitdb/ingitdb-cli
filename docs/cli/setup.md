### 🔹 setup` — initialise a new database directory _(not yet implemented)_

[Source Code](../../cmd/ingitdb/commands/setup.go)


```
ingitdb setup [--path=PATH]
```

| Flag          | Description                                                                     |
| ------------- | ------------------------------------------------------------------------------- |
| `--path=PATH` | Path to the directory to initialise. Defaults to the current working directory. |

**Examples:**

```shell
# 📘 Initialise a database in the current directory
ingitdb setup

# 🔁 Initialise a database at a specific path
ingitdb setup --path=/var/db/myapp
```

---

