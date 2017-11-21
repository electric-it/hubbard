# Hubbard

Hubbard is a local service designed to seamlessly interact with private, authenticated
instances of GitHub: Enterprise.

## Supported Protocols

* Git over HTTP
* Raw git assets from a given tree
* Direct download of release assets using the GitHub API

## Configuration

Set `GITHUB_URL` and `GITHUB_ACESS_TOKEN` before starting Hubbard.

You can also control the URL and access token by writing the values to
`/etc/hubbard/.hubbard.yml`.

Start Hubbard with `go run hubbard.go`.

## Installation

```bash
# Tap the keg
$ brew tap secc/secc https://github.build.ge.com/SECC/homebrew-secc

# Install Hubbard
# Hubbard is a HEAD-only cookbook because we rely on authentication with GitHub
$ brew install --HEAD hubbard
```

Homebrew will remind you to run the following:

```bash
$ hubbard configure --github-url=$YOUR_GITHUB_URL --github-access-token=$YOUR_GITHUB_ACCESS_TOKEN
```

Hubbard will now run as a service in the background, registered via Brew Services.

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

## Development

Make sure you have [Glide](https://github.com/Masterminds/glide):

```
brew install glide
```

Install your dependencies:

```
glide install
```

Run the server:

```
go run hubbard.go
```

You should also commit your built binaries:

```
./build
```

Why are we committing build artifacts?
Many contexts where we want to install Hubbard may not necessarily have access
to release assets.The whole idea behind Hubbard is to
avoid bad behavior like this in the future. Here, we assume bit of complexity
in one place so that this undesirable behavior doesn't propagate to
all other projects.

## Future work

* Single-line installation of Hubbard on CI
* Add support for YAML-based Viper configurations to manage multiple GH:E instances and authentication credentials by org / repo
