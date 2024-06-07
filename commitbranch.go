package cb

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/urfave/cli/v2"
)

func Main() {
	var rebaseParent string
	var shouldPush bool

	app := &cli.App{
		Name:  "cb",
		Usage: "Commit Branch",
		Commands: []*cli.Command{
			{
				Name:  "rebase",
				Usage: "Rebase a commit branch stack",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "parent",
						Value:       "main",
						Aliases:     []string{"p"},
						Usage:       "Parent branch to rebase from",
						Destination: &rebaseParent,
					},
					&cli.BoolFlag{
						Name:        "push",
						Usage:       "Push changes upstream",
						Destination: &shouldPush,
					},
				},
				Action: func(ctx *cli.Context) error {
					cwd, err := os.Getwd()
					if err != nil {
						return wrapErr(err, "error getting current working directory")
					}
					repo, err := git.PlainOpen(cwd)
					if err != nil {
						return wrapErr(err, "error opening repo")
					}
					// worktree, err := repo.Worktree()
					// if err != nil {
					// 	return wrapErr(err, "error getting worktree")
					// }

					// Fetch latest parent branch
					// TODO: Should look for upstream name
					upstream := "origin"
					err = execInteractive(fmt.Sprintf("git fetch %s %s", upstream, rebaseParent))
					if err != nil {
						return err
					}

					// homeDir, err := os.UserHomeDir()
					// if err != nil {
					// 	return err
					// }
					// sshKeyPath := filepath.Join(homeDir, ".ssh", "id_ed25519")
					// auth, err := ssh.NewPublicKeysFromFile("git", sshKeyPath, "")
					// if err != nil {
					// 	return wrapErr(err, "error loading ssh keys")
					// }
					// fmt.Printf("Pulling latest for: %s", rebaseParent)
					// err = worktree.Pull(&git.PullOptions{
					// 	SingleBranch:  true,
					// 	ReferenceName: plumbing.NewBranchReferenceName(rebaseParent),
					// 	Auth:          auth,
					// })
					// if err != nil && err != git.NoErrAlreadyUpToDate {
					// 	return wrapErr(err, "error fetching parent branch %s", rebaseParent)
					// }

					// Get Target branch to rebase
					targetBranch := ctx.Args().Get(0)
					// Default to current branch
					if targetBranch == "" {
						ref, err := repo.Head()
						if err != nil {
							return wrapErr(err, "error getting repo head")
						}
						refName := ref.Name()
						if !refName.IsBranch() {
							return fmt.Errorf("unable to infer target branch. not attached to an active branch.")
						}
						targetBranch = refName.Short()
					}

					// Validate Commit Branch Stack
					branches, err := findStackBranches(repo, targetBranch)
					if err != nil {
						return err
					}

					// Stash changes before rebases
					execInteractive("git stash")
					defer execInteractive("git stash pop")
					// Rebase Branches
					for i, stackBranch := range branches {
						var prevBranchName string
						var prevBranch *StackBranch
						if i == 0 {
							prevBranchName = fmt.Sprintf("%s/%s", upstream, rebaseParent)
						} else {
							prevBranch = branches[i-1]
							prevBranchName = prevBranch.branch.Name
						}

						// Remove the previous branch commit
						if prevBranch != nil {
							execInteractive(fmt.Sprintf("git rebase --onto %s %s %s", prevBranchName, prevBranch.commitSha, stackBranch.branch.Name))
						} else {
							execInteractive(fmt.Sprintf("git rebase %s %s", prevBranchName, stackBranch.branch.Name))
						}
					}

					// Push changes, if specified
					if shouldPush {
						branchNames := make([]string, 0, len(branches))
						for _, stackBranch := range branches {
							branchNames = append(branchNames, stackBranch.branch.Name)
						}
						execInteractive(fmt.Sprintf("git push  --atomic --force-with-lease %s %s", upstream, strings.Join(branchNames, " ")))
					}

					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type StackBranch struct {
	branch    *config.Branch
	commitSha string
}

// Finds stack branches in ascending order
func findStackBranches(repo *git.Repository, targetBranch string) (branches []*StackBranch, err error) {
	stackCount, branchBaseName, err := validateCBName(targetBranch)
	if err != nil {
		return
	}
	branches = make([]*StackBranch, 0, stackCount)

	// fmt.Printf("stackCount: %d\n", stackCount)
	for i := range stackCount {
		nextStackCount := i + 1
		branchName := branchBaseName + "-" + strconv.Itoa(nextStackCount)
		branch, err := repo.Branch(branchName)
		if err != nil {
			return branches, wrapErr(err, "error finding branch `%s` in working tree", branchName)
		}
		// Get last commit in branch.
		// Every branch should have one commit.
		logsToFetch := "1"
		if nextStackCount > 1 {
			logsToFetch = "2"
		}
		cmd := exec.Command("git", "log", "-n", logsToFetch, "--pretty=format:%H", branchName)
		// fmt.Printf("$ %s\n", cmd.String())
		logOut, err := cmd.Output()
		if err != nil {
			return branches, wrapErr(err, "error fetching branch logs")
		}
		// fmt.Printf("out: %s, err: %s\n", string(logOut), err)
		logs := strings.Split(strings.TrimSpace(string(logOut)), "\n")
		var branchCommitSha string
		if nextStackCount > 1 {
			currBranchSha := logs[0]
			prevBranchSha := logs[1]
			prevBranch := branches[i-1]
			// fmt.Printf("prevBranchSha: %s, currBranchSha: %s, prevBranch: %s, branch: %s\n", prevBranchSha, currBranchSha, prevBranch.branch.Name, branchName)

			if prevBranchSha != prevBranch.commitSha {
				// TODO: Could give the user the option to discord the commit
				// If they can verify nothing is missing.
				return branches, fmt.Errorf(
					"branch `%s` previous commit (%s) does not match previous branch `%s` commit (%s).\nEach branch should have 1 commit",
					branchName,
					prevBranchSha,
					prevBranch.branch.Name,
					prevBranch.commitSha,
				)
			}
			branchCommitSha = currBranchSha
		} else {
			branchCommitSha = logs[0]
		}
		// fmt.Printf("Branch sha: %s\n", branchCommitSha)

		branches = append(branches, &StackBranch{
			branch:    branch,
			commitSha: branchCommitSha,
		})
	}

	return
}

func validateCBName(name string) (stackCount int, branchBaseName string, err error) {
	lastIndex := strings.LastIndex(name, "-")
	if lastIndex < 1 {
		err = fmt.Errorf("branch `%s` does not match valid commit branch name", name)
		return
	}
	branchBaseName = name[:lastIndex]
	suffix := name[lastIndex+1:]

	stackCount, err = strconv.Atoi(suffix)
	if err != nil {
		err = wrapErr(err, "branch `%s` does not end in a number", name)
		return
	}

	if stackCount <= 0 {
		err = fmt.Errorf("branch `%s` stack count cannot be <= 0. got %d", name, stackCount)
		return
	}

	return
}

func wrapErr(err error, desc string, format ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(desc, format...), err)
}

func execInteractive(command string) error {
	// cmd := exec.Command("bash", "-c", "/usr/bin/python3")
	fmt.Printf("$ %s\n", command)
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
