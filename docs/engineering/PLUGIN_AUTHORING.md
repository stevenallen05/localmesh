# What's a plugin?

## Writing a plugin

In `plugin.toml`:

```toml
[identity]
module_name = "redis" 

# configs and env_vars are turned into global-scoped env vars
[[config]]
maxmemory = "256mb"                         # REDIS_MAXMEMORY="256mb" 

[[env_var]]
url = "{{ .ServiceName | upper }}_URL"      # env_var.url = REDIS_URL
```

## Using a plugin

In `project.toml`:

```toml
plugins = ["redis"]
```

In `deploy.toml`, override `[[config]]` values or rename `[[env_var]]` exports:

```toml
[redis]
maxmemory = "512mb"                         # REDIS_MAXMEMORY=512mb
url_as = "CACHE_URL"                        # REDIS_URL exported as CACHE_URL
```

Build and run:

```sh
$ localmesh build
$ docker compose up
```
