package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vubon/prv"
	"github.com/vubon/prv/internal/config"
	"github.com/vubon/prv/internal/context"
	"github.com/vubon/prv/internal/git"
	"github.com/vubon/prv/internal/matrix"
	"github.com/vubon/prv/internal/models"
	"github.com/vubon/prv/internal/output"
)

func printUsage() {
	fmt.Print(`prv — AI PR Review Context Engine & CLI

Usage:
  prv <command> [flags]

Commands:
  bundle      Gather PR metadata, diffs, and package context for AI review
  diagram     Generate Mermaid architecture and package dependency graph
  walkthrough Generate collapsible file-by-file changes walkthrough
  report      Format a review JSON result into markdown and copyable comments
  matrix      Print the 11-Dimension Engineering PR Review Matrix
  init        Generate a starter .prv.yaml and scaffold context directory with template.md
  version     Show prv CLI version

Examples:
  prv bundle --base staging            # Review current branch against staging
  prv bundle -B dev                    # Review current branch against dev
  prv bundle feat/payment staging      # Compare feature branch against staging
  prv bundle --branch feat/auth --base main
  prv bundle --staged                  # Review uncommitted staged changes
  prv bundle --dirty                   # Review uncommitted working tree + staged changes
  prv bundle --staged -o staged.md
  prv bundle --staged -c spec.md       # Review staged changes with design spec
  prv bundle --pr 42 -c 42.md          # Review PR #42 with reviewer context
  prv bundle --pr 42 --max-lines 1500  # Cap bundle diff at 1,500 lines (risk-tiered)
  prv bundle --pr https://github.com/owner/repo/pull/42 --json
  prv diagram --base staging           # Generate architecture diagram against staging
  prv diagram -B dev
  prv diagram --staged
  prv diagram --dirty
  prv walkthrough --base staging       # Generate file walkthrough against staging
  prv walkthrough -B dev
  prv walkthrough --staged
  prv walkthrough --dirty
  prv report --file review.json --pr 42 --post
  prv report --file review.json --pr 42 --post --inline
  prv report --file review.json --strict-evidence
  prv report --file review.json --copy
  prv init                             # Scaffold .prv.yaml and .prv/context/
  prv init --context-dir specs/reviews # Scaffold with custom context directory
  prv matrix
`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "bundle":
		runBundle(os.Args[2:])
	case "diagram":
		runDiagram(os.Args[2:])
	case "walkthrough":
		runWalkthrough(os.Args[2:])
	case "report":
		runReport(os.Args[2:])
	case "matrix":
		runMatrix()
	case "init":
		runInit(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("prv version %s\n", prv.Version())
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// parseFlagsAndPositionals parses flags from args while allowing flags to appear
// anywhere (before, between, or after positional arguments), overcoming Go's default
// flag.Parse behavior which stops parsing at the first positional argument.
func parseFlagsAndPositionals(fs *flag.FlagSet, args []string) []string {
	_ = fs.Parse(args)
	var positionals []string
	for rest := fs.Args(); len(rest) > 0; rest = fs.Args() {
		positionals = append(positionals, rest[0])
		_ = fs.Parse(rest[1:])
	}
	return positionals
}

// resolvePositionalArgs maps positional arguments to branch, base, or prID when flags are omitted.
func resolvePositionalArgs(posArgs []string, branch, base, prID *string) {
	if len(posArgs) == 0 {
		return
	}

	if len(posArgs) >= 2 {
		if *branch == "" {
			*branch = posArgs[0]
		}
		if *base == "" {
			*base = posArgs[1]
		}
		return
	}

	arg := strings.TrimSpace(posArgs[0])
	if arg == "" {
		return
	}

	// 1. Check if argument is a GitHub PR URL or PR number
	if *prID == "" && (strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") || isAllDigits(arg)) {
		*prID = arg
		return
	}

	// 2. If --base was explicitly specified, the positional argument is the feature branch
	if *base != "" && *branch == "" {
		*branch = arg
		return
	}

	// 3. If --branch was explicitly specified, the positional argument is the base branch
	if *branch != "" && *base == "" {
		*base = arg
		return
	}

	// 4. If neither was specified:
	// If the argument matches a standard integration base branch, treat it as the target base branch
	// so users can comfortably run: `prv bundle staging` or `prv bundle dev` from their current branch.
	commonBaseBranches := map[string]bool{
		"main": true, "master": true, "staging": true, "dev": true, "develop": true, "next": true, "trunk": true,
	}
	if commonBaseBranches[strings.ToLower(arg)] {
		*base = arg
		return
	}

	// Otherwise, default single positional argument to feature branch
	*branch = arg
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlag) Set(val string) error {
	*s = append(*s, val)
	return nil
}

func runBundle(args []string) {
	fs := flag.NewFlagSet("bundle", flag.ExitOnError)
	var branch, base, prID, repoPath, outputFile, configPath string
	var formatJSON, includeLocks, staged, dirty bool
	var maxLines int
	var contextFiles stringSliceFlag

	fs.BoolVar(&staged, "staged", false, "Bundle git staged changes (--cached) for local pre-commit review")
	fs.BoolVar(&staged, "s", false, "Alias for --staged")

	fs.BoolVar(&dirty, "dirty", false, "Bundle uncommitted working tree and staged changes for tracked files")
	fs.BoolVar(&dirty, "w", false, "Alias for --dirty")

	fs.StringVar(&branch, "branch", "", "Feature branch name")
	fs.StringVar(&branch, "b", "", "Alias for --branch")

	fs.StringVar(&base, "base", "", "Base target branch (defaults to main/master or .prv.yaml)")
	fs.StringVar(&base, "B", "", "Alias for --base")

	fs.StringVar(&prID, "pr", "", "PR number or full GitHub PR URL")

	fs.StringVar(&repoPath, "repo", ".", "Path to local git repository")
	fs.StringVar(&repoPath, "r", ".", "Alias for --repo")

	fs.StringVar(&configPath, "config", "", "Custom path to .prv.yaml configuration file")

	fs.Var(&contextFiles, "context-file", "Path to markdown context/spec file (repeatable)")
	fs.Var(&contextFiles, "c", "Alias for --context-file")

	fs.IntVar(&maxLines, "max-lines", 0, "Maximum total diff lines to include in bundle (0 = unlimited; risk-tiered)")

	fs.BoolVar(&formatJSON, "json", false, "Output bundle as raw JSON to stdout")

	fs.BoolVar(&includeLocks, "include-locks", false, "Include lockfiles and generated files in diff")

	fs.StringVar(&outputFile, "output", "", "Save bundle to specific file path")
	fs.StringVar(&outputFile, "out", "", "Alias for --output")
	fs.StringVar(&outputFile, "o", "", "Alias for --output")

	positionals := parseFlagsAndPositionals(fs, args)
	resolvePositionalArgs(positionals, &branch, &base, &prID)

	if staged && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --staged and --pr\n")
		os.Exit(1)
	}
	if dirty && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --dirty and --pr\n")
		os.Exit(1)
	}
	if base != "" && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --base and --pr\n")
		os.Exit(1)
	}
	if staged && dirty {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --staged and --dirty\n")
		os.Exit(1)
	}
	if maxLines < 0 {
		fmt.Fprintf(os.Stderr, "Error: --max-lines must be >= 0\n")
		os.Exit(1)
	}

	bundler := context.NewBundler(repoPath)
	opts := models.BundleOptions{
		Branch:       branch,
		BaseBranch:   base,
		PRIdentifier: prID,
		RepoPath:     repoPath,
		ConfigPath:   configPath,
		Staged:       staged,
		Dirty:        dirty,
		FormatJSON:   formatJSON,
		IncludeLocks: includeLocks,
		MaxDiffLines: maxLines,
		ContextFiles: contextFiles,
	}

	bundleCtx, err := bundler.PrepareBundle(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error preparing bundle: %v\n", err)
		os.Exit(1)
	}

	if formatJSON {
		jsonStr, err := bundler.ToJSON(bundleCtx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating JSON: %v\n", err)
			os.Exit(1)
		}
		if outputFile == "" {
			fmt.Println(jsonStr)
		}
	} else {
		output.PrintBundleSummary(bundleCtx)
		if outputFile == "" {
			fmt.Println(bundler.ToMarkdown(bundleCtx))
		}
	}

	if outputFile != "" {
		content := bundler.ToMarkdown(bundleCtx)
		if formatJSON {
			jsonContent, jsonErr := bundler.ToJSON(bundleCtx)
			if jsonErr != nil {
				fmt.Fprintf(os.Stderr, "Error generating JSON for file output: %v\n", jsonErr)
				os.Exit(1)
			}
			content = jsonContent
		}
		if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving bundle to %s: %v\n", outputFile, err)
			os.Exit(1)
		} else {
			fmt.Printf("Bundle successfully saved to %s\n", outputFile)
		}
	}
}

func runReport(args []string) {
	if err := executeReport(args, os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// parseFlagsStrict parses flags occurring anywhere before "--" while returning any flag parse error
// (including flag.ErrHelp) and preserving positional arguments.
func parseFlagsStrict(fs *flag.FlagSet, args []string) ([]string, error) {
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	var positionals []string
	for {
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		// Stop if Go's flag parser halted because of an explicit "--" terminator.
		consumed := len(args) - len(rest)
		if consumed > 0 && args[consumed-1] == "--" {
			positionals = append(positionals, rest...)
			break
		}
		positionals = append(positionals, rest[0])
		args = rest[1:]
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
	}
	return positionals, nil
}

func resolveReportConfigDir(repoPath, configPath string) string {
	if strings.TrimSpace(configPath) != "" {
		return repoPath
	}
	runner := git.NewGitRunner(repoPath)
	if runner.IsGitRepo() {
		if root, err := runner.GetRepoRoot(); err == nil && root != "" {
			return root
		}
	}
	return repoPath
}

func reportPRTarget(prID string, res *models.ReviewResult) (string, error) {
	target := strings.TrimSpace(prID)
	fromJSONURL := false
	if target == "" {
		target = strings.TrimSpace(res.PRURL)
		fromJSONURL = target != ""
		if target == "" && res.PRNumber != 0 {
			target = strconv.Itoa(res.PRNumber)
		}
	}
	if target == "" {
		return "", nil
	}

	if number, err := strconv.Atoi(target); !fromJSONURL && err == nil && number > 0 {
		return strconv.Itoa(number), nil
	}

	host, owner, repo, number, err := git.ParsePRURLWithHost(target)
	if err != nil || host == "" || owner == "" || repo == "" || number <= 0 ||
		(!strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "http://")) {
		return "", fmt.Errorf("invalid PR target %q, supply a positive PR number or a full GitHub PR URL", target)
	}
	if fromJSONURL && res.PRNumber != 0 && number != res.PRNumber {
		return "", fmt.Errorf("review JSON pr_url and pr_number refer to different PR numbers, supply --pr to select the target")
	}
	return target, nil
}

func executeReport(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var filePath, outputDir, repoPath, configPath string
	var copyClip, strictEvidence bool

	fs.StringVar(&filePath, "file", "", "Path to review JSON result file")
	fs.StringVar(&filePath, "f", "", "Alias for --file")

	fs.StringVar(&outputDir, "dir", "reviews", "Output directory for markdown reports")
	fs.StringVar(&outputDir, "d", "reviews", "Alias for --dir")

	fs.StringVar(&repoPath, "repo", ".", "Path to local git repository (for reading .prv.yaml)")
	fs.StringVar(&repoPath, "r", ".", "Alias for --repo")

	fs.StringVar(&configPath, "config", "", "Custom path to .prv.yaml configuration file")

	var prID string
	fs.StringVar(&prID, "pr", "", "Target PR number or full GitHub PR URL (required when using --post unless specified in JSON)")

	fs.BoolVar(&strictEvidence, "strict-evidence", false, "Fail if any BLOCKING issue lacks supporting evidence")

	var postComment bool
	fs.BoolVar(&postComment, "post", false, "Post generated review report directly to GitHub PR")
	fs.BoolVar(&postComment, "p", false, "Alias for --post")

	var inlineReview bool
	fs.BoolVar(&inlineReview, "inline", false, "Publish review with line-level inline comments on PR diff (GitHub Review API)")
	fs.BoolVar(&inlineReview, "i", false, "Alias for --inline")

	fs.BoolVar(&copyClip, "copy", false, "Copy formatted comments to clipboard")
	fs.BoolVar(&copyClip, "c", false, "Alias for --copy")

	positionals, err := parseFlagsStrict(fs, args)
	if err != nil {
		return err
	}

	if len(positionals) > 0 {
		if len(positionals) == 1 && prID == "" {
			arg := strings.TrimSpace(positionals[0])
			if isAllDigits(arg) || strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
				prID = arg
			} else {
				return fmt.Errorf("unexpected positional argument %q for report command", arg)
			}
		} else {
			return fmt.Errorf("unexpected positional argument(s) %v for report command", positionals)
		}
	}

	if filePath == "" {
		fs.Usage()
		return fmt.Errorf("--file is required for report command")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file %s: %w", filePath, err)
	}

	var res models.ReviewResult
	if err := json.Unmarshal(data, &res); err != nil {
		return fmt.Errorf("parsing review JSON: %w", err)
	}

	effectiveRepoPath := resolveReportConfigDir(repoPath, configPath)

	prTarget, err := reportPRTarget(prID, &res)
	if err != nil {
		return err
	}
	if postComment && prTarget == "" {
		return fmt.Errorf("--pr <number_or_url> is required when using --post (or set 'pr_number' / 'pr_url' in review JSON)")
	}
	bundler := context.NewBundler(effectiveRepoPath)
	cfg, prURL, cfgErr := bundler.ResolveReportConfig(configPath, res.Branch, res.BaseBranch, prTarget)
	if cfgErr != nil {
		return fmt.Errorf("loading .prv.yaml: %w", cfgErr)
	}

	strict := strictEvidence || (cfg != nil && cfg.Review.RequireEvidence)
	violations, evErr := output.ApplyEvidencePolicy(&res, strict)
	if evErr != nil {
		return evErr
	}
	if len(violations) > 0 {
		_, _ = fmt.Fprint(stderr, output.FormatEvidenceWarning(violations))
	}

	if prURL != "" {
		res.PRURL = prURL
		_, _, res.PRNumber, _ = git.ParsePRURL(prURL)
	}
	savedPath, err := output.SaveReportToFile(&res, outputDir)
	if err != nil {
		return fmt.Errorf("saving report: %w", err)
	}
	jsonPath := strings.TrimSuffix(savedPath, ".md") + ".json"
	_, _ = fmt.Fprintf(stdout, "Report saved to: %s\n", savedPath)
	_, _ = fmt.Fprintf(stdout, "JSON saved to: %s\n", jsonPath)

	output.PrintReviewSummary(&res)

	if postComment {
		ghClient := git.NewGitHubClient()
		payload := output.BuildReviewPayload(&res)

		// If inline was explicitly requested OR if the review contains line-level issues/suggestions,
		// publish as a GitHub PR Review with inline comments.
		if inlineReview || len(payload.Comments) > 0 {
			if reviewErr := ghClient.PostPRReview(prURL, payload, effectiveRepoPath); reviewErr != nil {
				// Gracefully fall back to standard PR comment if review creation failed
				reportContent := output.GenerateMarkdownReport(&res)
				if commentErr := ghClient.PostPRComment(prURL, reportContent, effectiveRepoPath); commentErr != nil {
					return fmt.Errorf("posting review to PR %s: %w", prURL, reviewErr)
				}
				_, _ = fmt.Fprintf(stdout, "Successfully published review report as a comment on PR %s (fallback from review API)!\n", prURL)
			} else {
				if len(payload.Comments) > 0 {
					_, _ = fmt.Fprintf(stdout, "Successfully published PR review with %d inline comments on PR %s (Verdict: %s)!\n", len(payload.Comments), prURL, payload.Event)
				} else {
					_, _ = fmt.Fprintf(stdout, "Successfully published PR review on PR %s (Verdict: %s)!\n", prURL, payload.Event)
				}
			}
		} else {
			reportContent := output.GenerateMarkdownReport(&res)
			if postErr := ghClient.PostPRComment(prURL, reportContent, effectiveRepoPath); postErr != nil {
				return fmt.Errorf("posting review comment to PR %s: %w", prURL, postErr)
			}
			_, _ = fmt.Fprintf(stdout, "Successfully published review report as a comment on PR %s!\n", prURL)
		}
	}

	if copyClip {
		count, err := output.CopyCommentsToClipboard(&res)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "Warning: could not copy to clipboard: %v\n", err)
		} else {
			_, _ = fmt.Fprintf(stdout, "Copied %d review comments to clipboard!\n", count)
		}
	}

	return nil
}

func runMatrix() {
	fmt.Println(matrix.GenerateMatrixMarkdown())
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	var repoPath, contextDir string

	fs.StringVar(&repoPath, "repo", ".", "Path to target repository")
	fs.StringVar(&repoPath, "r", ".", "Alias for --repo")
	fs.StringVar(&contextDir, "context-dir", ".prv/context", "Designated directory for reviewer context files")

	posArgs := parseFlagsAndPositionals(fs, args)

	dest := ".prv.yaml"
	if len(posArgs) > 0 {
		dest = posArgs[0]
	}

	targetConfig := dest
	if !filepath.IsAbs(targetConfig) && repoPath != "." && repoPath != "" {
		targetConfig = filepath.Join(repoPath, dest)
	}

	if err := config.WriteDefaultConfigWithContextDir(targetConfig, contextDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating configuration file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created starter configuration file: %s\n", targetConfig)
}

func runDiagram(args []string) {
	fs := flag.NewFlagSet("diagram", flag.ExitOnError)
	var branch, base, prID, repoPath, outputFile string
	var staged, dirty, markdownFence bool

	fs.BoolVar(&staged, "staged", false, "Generate diagram for git staged changes (--cached)")
	fs.BoolVar(&staged, "s", false, "Alias for --staged")

	fs.BoolVar(&dirty, "dirty", false, "Generate diagram for uncommitted working tree and staged changes")
	fs.BoolVar(&dirty, "w", false, "Alias for --dirty")

	fs.StringVar(&branch, "branch", "", "Feature branch name")
	fs.StringVar(&branch, "b", "", "Alias for --branch")

	fs.StringVar(&base, "base", "", "Base target branch (defaults to main/master or .prv.yaml)")
	fs.StringVar(&base, "B", "", "Alias for --base")

	fs.StringVar(&prID, "pr", "", "PR number or full GitHub PR URL")

	fs.StringVar(&repoPath, "repo", ".", "Path to local git repository")
	fs.StringVar(&repoPath, "r", ".", "Alias for --repo")

	fs.StringVar(&outputFile, "output", "", "Save diagram to specific file path")
	fs.StringVar(&outputFile, "out", "", "Alias for --output")
	fs.StringVar(&outputFile, "o", "", "Alias for --output")

	fs.BoolVar(&markdownFence, "markdown", false, "Wrap diagram inside ```mermaid markdown code fence")
	fs.BoolVar(&markdownFence, "m", false, "Alias for --markdown")

	positionals := parseFlagsAndPositionals(fs, args)
	resolvePositionalArgs(positionals, &branch, &base, &prID)

	if staged && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --staged and --pr\n")
		os.Exit(1)
	}
	if dirty && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --dirty and --pr\n")
		os.Exit(1)
	}
	if base != "" && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --base and --pr\n")
		os.Exit(1)
	}
	if staged && dirty {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --staged and --dirty\n")
		os.Exit(1)
	}

	bundler := context.NewBundler(repoPath)
	opts := models.BundleOptions{
		Branch:       branch,
		BaseBranch:   base,
		PRIdentifier: prID,
		RepoPath:     repoPath,
		Staged:       staged,
		Dirty:        dirty,
	}

	bundleCtx, err := bundler.PrepareBundle(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating diagram: %v\n", err)
		os.Exit(1)
	}

	diag := bundleCtx.ArchitectureDiagram
	if strings.TrimSpace(diag) == "" {
		fmt.Fprintf(os.Stderr, "No package architecture relationships found for the specified changes.\n")
		os.Exit(0)
	}

	content := diag
	if markdownFence {
		content = "```mermaid\n" + diag
		if !strings.HasSuffix(diag, "\n") {
			content += "\n"
		}
		content += "```\n"
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving diagram to %s: %v\n", outputFile, err)
			os.Exit(1)
		}
		fmt.Printf("Diagram successfully saved to %s\n", outputFile)
	} else {
		fmt.Print(content)
		if !strings.HasSuffix(content, "\n") {
			fmt.Println()
		}
	}
}

func runWalkthrough(args []string) {
	fs := flag.NewFlagSet("walkthrough", flag.ExitOnError)
	var branch, base, prID, repoPath, outputFile string
	var staged, dirty, formatJSON bool

	fs.BoolVar(&staged, "staged", false, "Generate walkthrough for git staged changes (--cached)")
	fs.BoolVar(&staged, "s", false, "Alias for --staged")

	fs.BoolVar(&dirty, "dirty", false, "Generate walkthrough for uncommitted working tree and staged changes")
	fs.BoolVar(&dirty, "w", false, "Alias for --dirty")

	fs.StringVar(&branch, "branch", "", "Feature branch name")
	fs.StringVar(&branch, "b", "", "Alias for --branch")

	fs.StringVar(&base, "base", "", "Base target branch (defaults to main/master or .prv.yaml)")
	fs.StringVar(&base, "B", "", "Alias for --base")

	fs.StringVar(&prID, "pr", "", "PR number or full GitHub PR URL")

	fs.StringVar(&repoPath, "repo", ".", "Path to local git repository")
	fs.StringVar(&repoPath, "r", ".", "Alias for --repo")

	fs.StringVar(&outputFile, "output", "", "Save walkthrough to specific file path")
	fs.StringVar(&outputFile, "out", "", "Alias for --output")
	fs.StringVar(&outputFile, "o", "", "Alias for --output")

	fs.BoolVar(&formatJSON, "json", false, "Output walkthrough as raw JSON")

	positionals := parseFlagsAndPositionals(fs, args)
	resolvePositionalArgs(positionals, &branch, &base, &prID)

	if staged && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --staged and --pr\n")
		os.Exit(1)
	}
	if dirty && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --dirty and --pr\n")
		os.Exit(1)
	}
	if base != "" && prID != "" {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --base and --pr\n")
		os.Exit(1)
	}
	if staged && dirty {
		fmt.Fprintf(os.Stderr, "Error: cannot specify both --staged and --dirty\n")
		os.Exit(1)
	}

	bundler := context.NewBundler(repoPath)
	opts := models.BundleOptions{
		Branch:       branch,
		BaseBranch:   base,
		PRIdentifier: prID,
		RepoPath:     repoPath,
		Staged:       staged,
		Dirty:        dirty,
	}

	bundleCtx, err := bundler.PrepareBundle(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating walkthrough: %v\n", err)
		os.Exit(1)
	}

	if len(bundleCtx.FileWalkthroughs) == 0 {
		fmt.Fprintf(os.Stderr, "No modified files found to generate walkthrough.\n")
		os.Exit(0)
	}

	var content string
	if formatJSON {
		bytes, err := json.MarshalIndent(bundleCtx.FileWalkthroughs, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		content = string(bytes)
	} else {
		content = output.FormatWalkthroughMarkdown(bundleCtx.FileWalkthroughs)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving walkthrough to %s: %v\n", outputFile, err)
			os.Exit(1)
		}
		fmt.Printf("Walkthrough successfully saved to %s\n", outputFile)
	} else {
		fmt.Println(content)
	}
}
