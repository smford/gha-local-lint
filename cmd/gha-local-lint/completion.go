package main

import (
	"fmt"
	"os"
	"strings"
)

func runCompletion(args []string) int {
	shell := "bash"
	if len(args) > 0 {
		shell = strings.ToLower(args[0])
	}

	switch shell {
	case "bash":
		fmt.Print(bashCompletionScript)
	case "zsh":
		fmt.Print(zshCompletionScript)
	case "fish":
		fmt.Print(fishCompletionScript)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell %q. Supported shells: bash, zsh, fish\n", shell)
		return 1
	}
	return 0
}

const bashCompletionScript = `_gha_local_lint() {
    local cur prev words cword
    _init_completion || return

    local commands="check simulate matrix cost graph expr completion version help"

    if [ $cword -eq 1 ]; then
        COMPREPLY=( $(compgen -W "${commands}" -- "$cur") )
        return 0
    fi

    case "${words[1]}" in
        check)
            local opts="--require-sha-pin --fail-on-warning --format --fix --config"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=( $(compgen -W "${opts}" -- "$cur") )
            else
                COMPREPLY=( $(compgen -f -X '!*.y*ml' -- "$cur") )
            fi
            ;;
        simulate)
            local opts="--event --ref --actor --repo --context-file --fail-job --mock-output --diff-against --detect-changes --interactive --format"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=( $(compgen -W "${opts}" -- "$cur") )
            else
                COMPREPLY=( $(compgen -f -X '!*.y*ml' -- "$cur") )
            fi
            ;;
        matrix)
            local opts="--job --format"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=( $(compgen -W "${opts}" -- "$cur") )
            else
                COMPREPLY=( $(compgen -f -X '!*.y*ml' -- "$cur") )
            fi
            ;;
        cost)
            local opts="--duration --runs --format"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=( $(compgen -W "${opts}" -- "$cur") )
            else
                COMPREPLY=( $(compgen -f -X '!*.y*ml' -- "$cur") )
            fi
            ;;
        graph)
            local opts="--format"
            if [[ "$cur" == -* ]]; then
                COMPREPLY=( $(compgen -W "${opts}" -- "$cur") )
            else
                COMPREPLY=( $(compgen -f -X '!*.y*ml' -- "$cur") )
            fi
            ;;
        expr)
            local opts="--context --interactive --format"
            COMPREPLY=( $(compgen -W "${opts}" -- "$cur") )
            ;;
        completion)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
            ;;
    esac
}
complete -F _gha_local_lint gha-local-lint
`

const zshCompletionScript = `#compdef gha-local-lint

_gha_local_lint() {
    local -a commands
    commands=(
        'check:Validate workflow files for DAG cycles, broken outputs, and security'
        'simulate:Dry-run simulate workflow execution against synthetic event contexts'
        'matrix:Inspect and expand matrix combinations'
        'cost:Estimate GitHub-hosted runner minutes and monthly cloud compute costs'
        'graph:Export workflow DAG as Mermaid or Graphviz DOT diagram'
        'expr:Evaluate GitHub Actions expressions and launch interactive REPL'
        'completion:Generate shell autocompletion script'
        'version:Display version, license, and repository information'
    )

    if (( CURRENT == 2 )); then
        _describe 'command' commands
        return
    fi

    case $words[2] in
        check)
            _arguments \
                '--require-sha-pin[Enforce 40-character SHA pinning]' \
                '--fail-on-warning[Exit with error on warnings]' \
                '--format[Output format (text, json, sarif, github)]:format:(text json sarif github)' \
                '--fix[Automatically remediate issues]' \
                '*:workflow file:_files -g "*.yml *.yaml"'
            ;;
        simulate)
            _arguments \
                '--event[Event name]:event:(push pull_request release workflow_dispatch schedule)' \
                '--ref[Git ref]:ref:' \
                '--actor[Actor username]:actor:' \
                '--format[Output format (text, json)]:format:(text json)' \
                '--interactive[Interactive simulation mode]' \
                '*:workflow file:_files -g "*.yml *.yaml"'
            ;;
        matrix)
            _arguments \
                '--job[Filter by job ID]:job:' \
                '--format[Output format (table, json)]:format:(table json)' \
                '*:workflow file:_files -g "*.yml *.yaml"'
            ;;
        cost)
            _arguments \
                '--duration[Average job duration in minutes]:duration:' \
                '--runs[Estimated monthly runs]:runs:' \
                '--format[Output format (table, json)]:format:(table json)' \
                '*:workflow file:_files -g "*.yml *.yaml"'
            ;;
        graph)
            _arguments \
                '--format[Output format (mermaid, dot)]:format:(mermaid dot)' \
                '*:workflow file:_files -g "*.yml *.yaml"'
            ;;
        expr)
            _arguments \
                '--context[Context JSON string or file]:context:' \
                '--interactive[Interactive REPL mode]' \
                '--format[Output format (text, json)]:format:(text json)'
            ;;
        completion)
            _values 'shell' bash zsh fish
            ;;
    esac
}

_gha_local_lint "$@"
`

const fishCompletionScript = `complete -c gha-local-lint -f

complete -c gha-local-lint -n "__fish_use_subcommand" -a check -d "Validate workflow files"
complete -c gha-local-lint -n "__fish_use_subcommand" -a simulate -d "Simulate workflow execution"
complete -c gha-local-lint -n "__fish_use_subcommand" -a matrix -d "Expand matrix combinations"
complete -c gha-local-lint -n "__fish_use_subcommand" -a graph -d "Export DAG as Mermaid or DOT"
complete -c gha-local-lint -n "__fish_use_subcommand" -a expr -d "Evaluate expressions & REPL"
complete -c gha-local-lint -n "__fish_use_subcommand" -a completion -d "Generate shell completion"
complete -c gha-local-lint -n "__fish_use_subcommand" -a version -d "Show version information"

complete -c gha-local-lint -n "__fish_seen_subcommand_from check" -l require-sha-pin -d "Enforce 40-character SHA pinning"
complete -c gha-local-lint -n "__fish_seen_subcommand_from check" -l fix -d "Auto-remediate issues"
complete -c gha-local-lint -n "__fish_seen_subcommand_from check" -l format -x -a "text json sarif github"
`
