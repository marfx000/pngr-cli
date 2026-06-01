package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"pngr.dev/cli/pkg/client"
)

func init() {
	rootCmd.AddCommand(applyCmd, diffCmd, dumpCmd)
}

// ─── flags ──────────────────────────────────────────────────────────────

var (
	stackFile   string
	stackDryRun bool
	stackYes    bool
	stackPrune  string // empty = no prune; "" with --prune flag set = all; "monitors,rules" etc.
	dumpFilter  string
	dumpMerge   string
)

// ─── apply ──────────────────────────────────────────────────────────────

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Reconcile the active org to match a Stack YAML file",
	Long: `Apply a Stack file (SPEC §2.11) to the active org.

  pngr apply -f stack.yaml             — parse → diff → preview → confirm → execute
  pngr apply -f stack.yaml --dry-run   — print the diff without applying
                                         (exit 0 if no changes, non-zero otherwise)
  pngr apply -f stack.yaml -y          — skip the confirmation prompt (CI)
  pngr apply -f stack.yaml --prune                — also delete resources not in the file
  pngr apply -f stack.yaml --prune=monitors,rules — scoped prune

The file may use ${ENV_VAR} placeholders for secrets (channel webhook
URLs, bot tokens). Each placeholder is resolved from the process
environment before the request is sent — the unrendered file is safe
to commit.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runApply(cmd, false /* not the diff alias */)
	},
}

// ─── diff (alias for apply --dry-run with tight output) ─────────────────

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Print the apply changeset without applying anything",
	Long: `Alias for "pngr apply --dry-run -f <file>" with a terser output (no
preview headers / no confirm prompt). Use in pre-commit hooks:

  pngr diff -f stack.yaml || exit 1

Exits 0 when there are no changes, non-zero when there would be — same
contract as 'apply --dry-run'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stackDryRun = true
		return runApply(cmd, true /* tight output */)
	},
}

// runApply is shared by `apply` and `diff`. tightOutput skips the preview
// headers + confirmation prompt (no point asking before something that
// doesn't mutate, and the pre-commit-hook user-case wants a clean diff).
func runApply(cmd *cobra.Command, tightOutput bool) error {
	if stackFile == "" {
		return fmt.Errorf("--file is required")
	}
	raw, err := os.ReadFile(stackFile)
	if err != nil {
		return fmt.Errorf("reading %s: %w", stackFile, err)
	}

	expanded, missing := expandEnv(string(raw))
	if len(missing) > 0 {
		return fmt.Errorf("undefined environment variables referenced by %s: %s",
			stackFile, strings.Join(missing, ", "))
	}

	var stack client.Stack
	if err := yaml.Unmarshal([]byte(expanded), &stack); err != nil {
		return fmt.Errorf("parsing %s: %w", stackFile, err)
	}

	c, orgID, err := newClientWithOrg()
	if err != nil {
		return err
	}

	opts := client.ApplyOptions{DryRun: stackDryRun}
	if cmd.Flags().Changed("prune") {
		opts.Prune = parsePrune(stackPrune)
	}

	// On a real apply, do a dry-run first so the user sees what's about
	// to land before confirming. Skips when --yes is set OR when this
	// is the diff alias (already a dry run).
	if !stackDryRun && !stackYes {
		preview, err := c.Stack.Apply(context.Background(), orgID, &stack,
			client.ApplyOptions{DryRun: true, Prune: opts.Prune})
		if err != nil {
			return err
		}
		printChangeset(cmd.OutOrStdout(), preview.Changeset, false)
		if !preview.Changeset.HasChanges() {
			fmt.Fprintln(cmd.OutOrStdout(), "no changes — exiting without apply")
			return nil
		}
		if !confirm(cmd, "apply these changes?") {
			fmt.Fprintln(cmd.OutOrStdout(), "aborted")
			return nil
		}
	}

	result, err := c.Stack.Apply(context.Background(), orgID, &stack, opts)
	if err != nil {
		return err
	}
	printChangeset(cmd.OutOrStdout(), result.Changeset, tightOutput)

	// dry-run / diff: non-zero exit when there'd be changes, so the
	// command works as a CI guard.
	if stackDryRun {
		if result.Changeset.HasChanges() {
			os.Exit(2) //nolint:gocritic
		}
		return nil
	}

	// Partial-success: per-row errors land in change.Error; surface them
	// as a non-zero exit so CI doesn't silently mark the run green.
	for _, ch := range result.Changeset.Changes {
		if ch.Error != "" {
			return fmt.Errorf("apply completed with errors — see changeset above")
		}
	}
	return nil
}

// ─── dump ───────────────────────────────────────────────────────────────

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Export the active org as a Stack YAML file",
	Long: `Render the active org as a Stack document on stdout. The output is
guaranteed to be valid input to 'pngr apply' against the same org
with zero changes (round-trip property — §2.11.5).

Channel secrets are emitted as ${SECRET_NAME} placeholders; wire the
real values in your secret store / environment.

  pngr dump > stack.yaml
  pngr dump --filter monitors,rules     — partial export
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		stack, err := c.Stack.Dump(context.Background(), orgID)
		if err != nil {
			return err
		}
		if dumpFilter != "" {
			applyDumpFilter(stack, dumpFilter)
		}
		// TODO: --merge support is documented in §2.11.5 but punted to a
		// follow-up — gopkg.in/yaml.v3's comment preservation requires
		// keeping the original yaml.Node tree, not the decoded Go
		// struct we have. Calling it out explicitly so users don't get
		// silent no-op behavior on the flag.
		if dumpMerge != "" {
			return fmt.Errorf("--merge is not implemented yet; use a plain `pngr dump > stack.yaml` and edit by hand")
		}
		out, err := yaml.Marshal(stack)
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(out)
		return err
	},
}

func applyDumpFilter(s *client.Stack, raw string) {
	keep := map[string]bool{}
	for _, p := range strings.Split(raw, ",") {
		keep[strings.TrimSpace(strings.ToLower(p))] = true
	}
	if !keep["channels"] && !keep["channel"] {
		s.Spec.Channels = nil
	}
	if !keep["monitors"] && !keep["monitor"] {
		s.Spec.Monitors = nil
	}
	if !keep["rules"] && !keep["rule"] {
		s.Spec.Rules = nil
	}
}

func init() {
	applyCmd.Flags().StringVarP(&stackFile, "file", "f", "", "Stack YAML file path (required)")
	applyCmd.Flags().BoolVar(&stackDryRun, "dry-run", false, "compute the diff but don't apply it")
	applyCmd.Flags().BoolVarP(&stackYes, "yes", "y", false, "skip the confirmation prompt (for CI)")
	// --prune is a tri-state flag: absent = no prune, present-no-value
	// = prune all kinds, present-with-CSV = prune those kinds.
	// cobra/spf13 doesn't model that cleanly, so we use a string default
	// and read cmd.Flags().Changed() at runtime.
	applyCmd.Flags().StringVar(&stackPrune, "prune", "", "kinds to prune: comma list of channels|monitors|rules (no value = all kinds)")
	applyCmd.Flag("prune").NoOptDefVal = "all"
	_ = applyCmd.MarkFlagRequired("file")

	diffCmd.Flags().StringVarP(&stackFile, "file", "f", "", "Stack YAML file path (required)")
	diffCmd.Flags().StringVar(&stackPrune, "prune", "", "kinds to prune: comma list of channels|monitors|rules (no value = all kinds)")
	diffCmd.Flag("prune").NoOptDefVal = "all"
	_ = diffCmd.MarkFlagRequired("file")

	dumpCmd.Flags().StringVar(&dumpFilter, "filter", "", "comma list of kinds to include: channels,monitors,rules")
	dumpCmd.Flags().StringVar(&dumpMerge, "merge", "", "preserve comments/key-order from this file (not yet implemented)")
}

func parsePrune(raw string) []client.ChangeKind {
	if raw == "" || raw == "all" {
		return []client.ChangeKind{client.KindChannel, client.KindMonitor, client.KindRule}
	}
	var out []client.ChangeKind
	for _, p := range strings.Split(raw, ",") {
		switch strings.TrimSpace(strings.ToLower(p)) {
		case "channel", "channels":
			out = append(out, client.KindChannel)
		case "monitor", "monitors":
			out = append(out, client.KindMonitor)
		case "rule", "rules":
			out = append(out, client.KindRule)
		}
	}
	return out
}

// ─── ${VAR} interpolation ───────────────────────────────────────────────

// envVarPattern matches `${NAME}` where NAME is alnum + underscore. Plain
// `$NAME` is intentionally NOT supported — too many false positives in
// YAML strings (URLs, regex patterns, body matchers).
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnv substitutes ${ENV_VAR} occurrences and returns (expanded, missing).
// A var that isn't in the environment is left as the literal ${NAME}
// in the output but reported in `missing` so the caller can fail loudly
// — a silent empty-string substitution would land a blank webhook URL.
func expandEnv(s string) (string, []string) {
	missing := []string{}
	seen := map[string]bool{}
	out := envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-1]
		v, ok := os.LookupEnv(name)
		if !ok {
			if !seen[name] {
				missing = append(missing, name)
				seen[name] = true
			}
			return match
		}
		return v
	})
	sort.Strings(missing)
	return out, missing
}

// ─── changeset rendering ────────────────────────────────────────────────

// printChangeset renders the changeset in a compact, scannable form.
// `tight` omits headers and totals — used by `pngr diff` for pre-commit
// hooks where every line of output is noise.
func printChangeset(w io.Writer, cs *client.Changeset, tight bool) {
	// Group by kind so users see channels → monitors → rules — same
	// order the apply executes in.
	byKind := map[client.ChangeKind][]client.Change{}
	for _, c := range cs.Changes {
		byKind[c.Kind] = append(byKind[c.Kind], c)
	}
	order := []client.ChangeKind{client.KindChannel, client.KindMonitor, client.KindRule}
	totals := map[client.ChangeOp]int{}

	for _, kind := range order {
		changes := byKind[kind]
		if len(changes) == 0 {
			continue
		}
		if !tight {
			fmt.Fprintf(w, "\n%s:\n", strings.Title(string(kind))+"s") //nolint:staticcheck
		}
		for _, c := range changes {
			switch c.Op {
			case client.OpCreate:
				fmt.Fprintf(w, "  + %s/%s\n", c.Kind, c.Name)
			case client.OpDelete:
				fmt.Fprintf(w, "  - %s/%s\n", c.Kind, c.Name)
			case client.OpUpdate:
				fmt.Fprintf(w, "  ~ %s/%s\n", c.Kind, c.Name)
				if !tight && len(c.Diff) > 0 {
					keys := make([]string, 0, len(c.Diff))
					for k := range c.Diff {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						fmt.Fprintf(w, "      %s: %v → %v\n", k, leftOf(c.Diff[k]), rightOf(c.Diff[k]))
					}
				}
			case client.OpNoop:
				if !tight {
					fmt.Fprintf(w, "    %s/%s (no-op)\n", c.Kind, c.Name)
				}
			}
			if c.Error != "" {
				fmt.Fprintf(w, "      error: %s\n", c.Error)
			}
			totals[c.Op]++
		}
	}
	if !tight {
		fmt.Fprintf(w, "\nsummary: %d create, %d update, %d delete, %d no-op\n",
			totals[client.OpCreate], totals[client.OpUpdate], totals[client.OpDelete], totals[client.OpNoop])
	}
}

// leftOf / rightOf pull the [old, new] pair the server emits as a 2-element
// slice in Change.Diff. Falls back to printing the raw value if the
// shape is unexpected, so an additive diff field doesn't crash output.
func leftOf(v any) any {
	if s, ok := v.([]any); ok && len(s) == 2 {
		return s[0]
	}
	return v
}
func rightOf(v any) any {
	if s, ok := v.([]any); ok && len(s) == 2 {
		return s[1]
	}
	return v
}

// confirm prompts the user. Honors --output=json (skips prompt, assumes
// no) so a piped run doesn't hang waiting for stdin.
func confirm(cmd *cobra.Command, prompt string) bool {
	if globalConfig.Output != "table" {
		return false
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "%s [y/N]: ", prompt)
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
