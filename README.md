Caddy Git Filesystem
====

This plugin allows you to serve files from a git repository directory by cloning it in-memory at Caddy boot.

## Installation

Install the plugin using the `xcaddy`:

```shell
xcaddy build --with github.com/mohammed90/caddy-git-fs
```

## Usage


```caddyfile
{
	filesystem nginx-repo git "https://github.com/caddyserver/nginx-adapter"
}
example.com {
	file_server {
		fs nginx-repo
	}
}

```
