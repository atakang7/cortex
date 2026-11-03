# cortex

cortex is a terminal coding agent built on [axon](https://github.com/atakang7/axon). It runs model and tool turns in your working directory, saves sessions, and leaves the transcript in your terminal scrollback.

## Build

Requires Go 1.26.2 or a compatible newer release.

```sh
git clone https://github.com/atakang7/cortex.git
cd cortex
go build -o cortex ./cmd/cortex
./cortex --version
```

On Windows, build with `go build -o cortex.exe ./cmd/cortex`. Native Windows builds are covered by CI.

To install from the module instead:

```sh
go install github.com/atakang7/cortex/v2/cmd/cortex@latest
```

Linux and macOS release binaries are available from [GitHub Releases](https://github.com/atakang7/cortex/releases). The repository also includes `scripts/install.sh` for installing a release into `~/.local/bin`.

## Configure

cortex selects a provider and model. Axon supplies the provider endpoint, credentials, and runtime settings.

For a first OpenRouter run on Linux or macOS:

```sh
mkdir -p ~/.config/axon ~/.config/cortex
cat > ~/.config/axon/axon.yaml <<'YAML'
providers:
  openrouter:
    base_url: https://openrouter.ai/api
    api_key: ${OPENROUTER_API_KEY}
    models:
      deepseek/deepseek-v3.2:
YAML
cp .env.example ~/.config/axon/.env
chmod 600 ~/.config/axon/.env
printf 'provider: openrouter\nmodel: deepseek/deepseek-v3.2\n' > ~/.config/cortex/config.yaml
```

Replace `replace-with-your-key` in `~/.config/axon/.env` with your OpenRouter key before starting `./cortex`. Do not commit the credential file.

cortex reads `~/.config/cortex/config.yaml` and then `./cortex.yaml`, with project fields taking precedence. `LLM_PROVIDER` and `LLM_MODEL` override both. `--config PATH` reads only the named cortex config plus those overrides. Axon reads `~/.config/axon/axon.yaml` and its adjacent `.env`.

Optional cortex fields are `name`, `system_prompt`, `pruner`, `explorer`, and `mcp_servers`. A `system_prompt` value can be literal text or a file path, including `~/...`. See [configuration details](https://atakang7.github.io/cortex/docs/guides/configuration/).

## Use

```sh
./cortex
./cortex --prompt "list the TODOs in this repo"
./cortex --config reviewer.yaml --prompt "review the last commit"
./cortex --acp
./cortex --stdio
```

Without flags, cortex starts an interactive transcript in the current directory. `--prompt` runs one turn, prints the answer to stdout and sends progress and errors to stderr. `--acp` and `--stdio` expose separate editor protocols over standard input and output.

The agent loop, session storage, tool execution, and context checkpoints live in axon. cortex chooses the model and prompt, connects optional MCP servers, and renders the events. Run traces are written under the OS user cache directory and include model messages and tool output.

## Check changes

```sh
go build ./...
go vet ./...
go test ./...
cd docs && npm ci && npm run build
```

The docs site uses Node.js 24. Pull requests run Go tests on Linux and Windows, source builds on macOS, and an Astro docs build when docs change.

## Limits

- A non-interactive turn stops after at most 120 agent iterations. Interactive turns rely on user cancellation.
- The agent can execute shell commands and edit files in the working directory. Run it on code you trust, or use an isolated workspace.
- Provider access requires a configured model endpoint. The default OpenRouter example needs network access and an API key.
- Traces can contain repository contents and sensitive tool output. Keep the user cache private.

Full command and configuration reference: [cortex docs](https://atakang7.github.io/cortex/).
