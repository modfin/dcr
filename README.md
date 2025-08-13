# dcr
A wrapper for docker compose

## Configuration
This project uses two configuration files: `.dcrgroups` and `.dcrconfig.yaml`, both residing in the same directory as the Docker compose file.
- `.dcrgroups`: This file defines groups of services that can be started or stopped together.
- `.dcrconfig.yaml`: This file is meant for general configurations. At the moment, it only includes settings for running CLI-based commands run before each service is started.

### .dcrconfig.yaml
Under the top-level property `action-config`, you can specify a set of CLI commands that are run for each service in conjunction with a `dcr` command. At the moment, only `pre-start-actions` (actions that are run when `up`ing a service) are supported. The actions are run in the order they are specified in the `action` list:
```yaml
action-config:
    pre-start-actions:
      actions:
        - echo "Running pre-start command for service"
        - echo "This can be any command you want to run before starting the service"
```

#### -- Enable (`enabled: true`) --
The actions are disabled by default. To enable them, set `enabled` to `true`:
```yaml
action-config:
  pre-start-actions:
    enabled: true # Must be true for actions to be run.
    actions:
      - echo "Running pre-start command for service"
      - echo "This can be any command you want to run before starting the service"
```

#### -- Showing Output (`verbose: true`) --
Output from actions is suppressed by default. To show the output, set `verbose` to `true`:
```yaml
action-config:
  pre-start-actions:
    enabled: true
    verbose: true # Now the actions below will echo output to the terminal
    actions:
      - echo "Running pre-start command for service"
      - echo "This can be any command you want to run before starting the service"
```

#### -- Using Service Names (`%s`) --
To interpolate the service name(s) for which the command is run, include `%s` in the command string. Assume that you start `service-1` and `service-2` with the following configuration:
```yaml
action-config:
  pre-start-actions:
    enabled: true
    verbose: true
    actions:
      - echo "Running pre-start command for service %s"
```

This would output:
```
Running pre-start command for service service-1
Running pre-start command for service service-2
```

#### -- Filtering Services (`filter` regex) --
Specific services can be targeted by specifying a `filter` regex. Assume that you start `postgres-db` and `redis` with the following configuration:
```yaml
action-config:
  pre-start-actions:
    enabled: true
    verbose: true
    filter: "^postgres"
    actions:
      - echo "Running pre-start command for service %s"
```

This would output:
```
Running pre-start command for service postgres-db
```

#### -- Reusable Action Templates --
You can specify a template name together with the `type: "template"` for a CLI command defined under `action-templates` for reusability:
```yaml
action-config:
  pre-start-actions:
    enabled: true
    verbose: true
    actions:
      - action: "list"
        type: "template"
  action-templates:
    list:
      action: "echo '%s'"
      description: "List available services" # Only used for documentation purposes at the moment 
```

### Zdap Integration
This project is integrated with Zdap, a command-line interface (CLI) tool used by developers to create, attach, and detach database instances from their local Docker environment.

The Zdap configurations are specified under `integrations` -> `zdap` and shares the same properties as `action-config` but also comes with exclusive features.

Unlike `action-config`, the `pre-start-actions` specified under `zdap` run only for the dependencies that are listed under `depends_on` in `docker-compose.yml` per service started by DCR. For example, starting a service `service-1` that depends on `zdap-db-1` and `zdap-db-2` will run the pre-start actions only for `zdap-db-1` and `zdap-db-2`, but not for `service-1`.

#### -- Enable (`--zdap`) --
Like the `pre-start-actions` in `action-config`, they are disabled by default and can be enabled by setting `enabled` to `true`. You may also enable them by passing the flag `--zdap` when running `up`, e.g.
```
dcr --zdap . up -d service-1
```

#### -- Only Uncloned Dependencies --
To only run actions for uncloned dependencies, set `only-uncloned-dependencies` to `true`. This will skip running actions for dependencies that have already been cloned. For example, if you have a service that depends on a Zdap database instance, the pre-start actions will only run if the database instance has not been cloned yet.
```yaml
integrations:
  zdap:
    only-uncloned-dependencies: true # Only run pre-start actions for uncloned Zdap dependencies
    pre-start-actions:
      enabled: true
      actions:
        - action: "detach"
          type: "template"
        - action: "attach"
          type: "template"
    action-templates:
      detach:
        action: "zdap detach %s"
        description: "Detach the zdap resource"
      attach:
        action: "zdap attach %s"
        description: "Attach the zdap resource"
```

#### -- Ignored Resources --
You may also specify a list of services to be ignored under `ignored-resources`:
```yaml
integrations:
  zdap:
    pre-start-actions:
      enabled: true
      actions:
        - action: "detach"
          type: "template"
        - action: "attach"
          type: "template"
    ignored-resources:
      - "zdap-db-1" # This resource will be ignored
    action-templates:
      detach:
        action: "zdap detach %s"
        description: "Detach the zdap resource"
      attach:
        action: "zdap attach %s"
        description: "Attach the zdap resource"
```

#### -- Zdap Resource Overrides --
If the service name in `docker-compose.yml` does not coincide with the Zdap resource name, you can override it by specifying the `name` of the Docker compose service, and the `resource-name` of the Zdap resource under `resource-name-overrides`:
```yaml
integrations:
  zdap:
    pre-start-actions:
      enabled: true
      actions:
        - action: "detach"
          type: "template"
        - action: "attach"
          type: "template"
    action-templates:
      detach:
        action: "zdap detach %s"
        description: "Detach the zdap resource"
      attach:
        action: "zdap attach %s"
        description: "Attach the zdap resource"
    resource-name-overrides:
      - name: "db-1" # The name of the Docker compose service
        resource-name: "zdap-db-1" # The name of the Zdap resource to rename the Docker compose service to
```