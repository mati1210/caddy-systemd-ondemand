# !! WIP !!

# Start systemd services on demand with caddy

## Installation
install caddy with this module
```sh
xcaddy build --with=github.com/mati1210/caddy-systemd-ondemand
```

## Configuration

if your caddy server is not running as root, you'll need to set up a polkit rule so it has the privileges to start services
place a .rules file like this under /etc/polkit-1/rules.d

```js
polkit.addRule(function (action, subject) {
    if (action.id == "org.freedesktop.systemd1.manage-units"
        && subject.user == "caddy"
        && action.lookup("verb") == "start"
        && (action.lookup("unit") == "foo.service" || action.lookup("unit") == "bar.service")
    ) {
        return polkit.Result.YES;
    }
})
```

then, on your caddyfile, add it before your `reverse_proxy` directive
```caddyfile
foo.example.org {
    systemd_service foo.service {
        # (optional) how long the service takes to start, in avg
        startup_time 5s

        # (optional) replace the default "service is starting" http page with something else
        body "wait!"
    }
    reverse_proxy localhost:8888
}
```