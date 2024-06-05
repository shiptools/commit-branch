package cb

import (
	"fmt"
	"log"
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
					err = execInteractive(fmt.Sprintf("git fetch origin %s", rebaseParent))
					if err != nil {
						return err;
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
					// branchIter, err := repo.Branches()
					// if err != nil {
					// 	return wrapErr(err, "all branched error")
					// }
					// cfg, _ := repo.Config()
					// fmt.Printf("config: %+v\n", cfg)
					// branchIter.ForEach(func(r *plumbing.Reference) error {
					// 	fmt.Printf("name: %s, type: %s\n", r.Name(), r.Type())
					// 	return nil;
					// })

					branches, err := findStackBranches(repo, targetBranch)
					if err != nil {
						return err
					}

					// Merge Branches
					branch := branches[0]
					// Stash changes before rebases
					execInteractive("git stash")
					defer execInteractive("git stash pop")
					println(fmt.Sprintf("git rebase main %s", branch.Name))
					execInteractive(fmt.Sprintf("git rebase main %s", branch.Name))

					// worktree.Fet
					// mainBranch := plumbing.NewBranchReferenceName(rebaseParent)
					// err = worktree.Checkout(&git.CheckoutOptions{
					// 	Branch: mainBranch,
					// })

					// branch, err := repo.Branch(rebaseParent)
					// branch.Rebase
					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

// Finds stack branches in ascending order
func findStackBranches(repo *git.Repository, targetBranch string) (branches []*config.Branch, err error) {
	stackCount, branchBaseName, err := validateCBName(targetBranch)
	if err != nil {
		return
	}

	// fmt.Printf("stackCount: %d\n", stackCount)
	for i := range stackCount {
		nextStackCount := i + 1
		branchName := branchBaseName + "-" + strconv.Itoa(nextStackCount)
		branch, err := repo.Branch(branchName)
		if err != nil {
			return branches, wrapErr(err, "error finding branch `%s` in working tree", branchName)
		}
		branches = append(branches, branch)
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
	return fmt.Errorf("%s: %w", fmt.Sprintf(desc, format), err)
}

func execInteractive(command string) error {
	// cmd := exec.Command("bash", "-c", "/usr/bin/python3")
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
