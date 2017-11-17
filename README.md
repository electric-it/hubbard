# Hubbard

Hubbard is a local service designed to seamlessly interact with private, authenticated
instances of GitHub: Enterprise.

## Supported Protocols

* Git over HTTP
* Raw git assets from a given tree
* Direct download of release assets using the GitHub API

## Configuration

Set `GITHUB_URL` and `GITHUB_ACESS_TOKEN` when starting Hubbard.

Start Hubbard with `go run server.go`.

## Usage

Hubbard serves at `localhost:41968`.
For dependency management over the supported protocols, you can now replace
the GitHub Enterprise URL you reference with this address.
Hubbard will do the work of proxying over your authentication credentials,
and, in the case of interacting with release assets, retrieve the appropriate
asset via the GitHub API and provide the necessary download context.

This means that installing assets from `npm`, `pip`, `bundler` or any other package
management solution that supports compressed flat-file artifacts operates
seamlessly.

## Future work

* Add support for YAML-based Viper configurations to manage multiple GH:E instances and authentication credentials by org / repo
* Add a CLI using Cobra that lets you:
* * manage configurations
* * start/stop the service
* * configure `/etc/hosts` to alias `localhost` to `hubbard` for more clarity
