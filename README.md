# KlyraDB

> Manage isolated PostgreSQL, MySQL, MariaDB and Redis instances on Linux.
> No Docker. No config files. Just click and run.

---

## Install

```bash
sudo snap install klyradb
```

That's it. No dependencies, no setup.

---

## What it does

- Create multiple database instances, each on its own port
- Start and stop them individually
- Copy the connection URI with one click
- Works with whatever database clients you already have

**Supported engines and versions:**

| Engine     | Versions          |
|------------|-------------------|
| PostgreSQL | 14, 15, 16, 17    |
| MySQL      | 8.0, 8.4          |
| MariaDB    | 10.6, 10.11, 11.4 |
| Redis      | 7.2, 7.4          |

> The database binaries (postgres, mysqld, redis-server) must be installed on your system. KlyraDB manages them — it does not bundle them.

---

## License

MIT
