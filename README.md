# mcp-imap
A simple no-thrills IMAP adapter to MCP.

## Get Started
You may either use the container [ghcr.io/meschbach/mcp-imap:latest](https://github.com/meschbach/mcp-imap/pkgs/container/mcp-imap) (recommended)
or download a copy of the binary from the [releases page](https://github.com/meschbach/mcp-imap/releases).

Following environment variables are required:
- `MCP_USERNAME` - Your user inbox, for example `meschbach`
- `MCP_PASSWORD` - See below for Gmail, otherwise your password.
- `MCP_SERVER` - `imap.gmail.com`

## Using your Google Account
Gmail will require you to use an _app password_ to access your account via IMAP.  Setup one at [https://myaccount.google.com/apppasswords](https://myaccount.google.com/apppasswords).