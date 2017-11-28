# Hubbard

Hubbard is a local service designed to seamlessly interact with private, authenticated
instances of GitHub: Enterprise.

## Supported Protocols

* Git over HTTP
* Raw git assets from a given tree
* Direct download of release assets using the GitHub API

## Configuration

Set `GITHUB_URL` and `GITHUB_ACESS_TOKEN` before starting Hubbard.

The **Installation** section will provide guidelines for configuring
a personal access token in GitHub.

The GitHub URL is the URL of the GitHub instance you're working with.
For instance, the build.GE GitHub URL is `https://github.build.ge.com`.

You can also control the URL and access token by writing the values to
`/etc/hubbard/.hubbard.yml`.

Start Hubbard with `go run hubbard.go`.

## Installation
First, you will need an access key. Go to the **Settings** page of
the GitHub instance you plan to work with. From there, you should be able to
view your **Personal Access Tokens**:

![Personal Access Token](https://github.build.ge.com/SECC/hubbard/blob/master/doc/personal_access_token.png)

You can then choose to **Generate a New Token**:

![Generate New Token](https://github.build.ge.com/SECC/hubbard/blob/master/doc/generate_new_token.png)

Select the permissions you believe are applicable. Once you've created the access
token, you should record it in a text file or note so that you can use it for configuring
Hubbard in the steps below.

### OS X
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

If you are not prompted to allow a particular port to open, you should
consider running hubbard in the foreground instead using `hubbard run-fg`.

### Linux
Hubbard is also designed to work as a service in a Linux environment.

As a superuser, with `GITHUB_URL` and `GITHUB_ACCESS_TOKEN` passed into the
environment, you should be able to configure hubbard as a service using:

```
./register-hubbard-service
```

### Windows
We have a binary for Windows, but we are ***actively seeking collaborators***
for validating that the binary works, and for correctly registering with Windows.

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
# Running make will create binaries in ./pkg for each supported platform
make
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
