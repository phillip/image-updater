/*
Copyright © 2022 Phil Wolf

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "image-updater <github_owner>/<github_repo_name>/<file_path> <new_tag>",
	Args:  cobra.ExactArgs(2),
	Short: "Tool to update image tags in Kubernetes manifests",
	Long: `Tool to update image tags in Kubernetes manifests

Example:
	image-updater -l tag phillip/image-updater-test/values.yaml v0.2`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		githubToken := os.Getenv("GITHUB_TOKEN")
		if githubToken == "" {
			fmt.Fprintf(os.Stderr, "Error: %v", "GITHUB_TOKEN must be set in the shell environment.")
			os.Exit(1)
		}

		if !strings.Contains(args[0], "/") {
			fmt.Fprintf(os.Stderr, "Error: %v", "github_file_path must be <github_owner>/<github_repository>/<file_path_in_repo>")
			os.Exit(1)
		}

		parts := strings.Split(args[0], "/")
		owner := parts[0]
		repo := parts[1]
		filePath := strings.Join(parts[2:], "/")
		label, _ := cmd.Flags().GetString("label")
		newTag := args[1]
		branchName, _ := cmd.Flags().GetString("branch")
		nth, _ := cmd.Flags().GetInt("nth")
		message, _ := cmd.Flags().GetString("message")
		tLabel, _ := cmd.Flags().GetString("timestamp")
		tNth, _ := cmd.Flags().GetInt("tnth")

		variables := map[string]interface{}{
			"owner":    githubv4.String(owner),
			"name":     githubv4.String(repo),
			"filePath": githubv4.String("HEAD:" + filePath),
		}

		client := getClient(githubToken)

		testLogin, _ := cmd.Flags().GetBool("testLogin")
		if testLogin {
			getLoginTest(client)
		} else {
			content := getFileContent(client, variables)

			newContent := replaceTag(content, label, newTag, nth)

			if tLabel != "" {
				ts := time.Now().UTC().Format(time.RFC3339)
				newContent = replaceTag(newContent, tLabel, ts, tNth)
			}

			delete(variables, "filePath")
			commitMessage := ""
			if message != "" {
				commitMessage = message
			} else {
				commitMessage = "Updated " + label + " to " + newTag + " in " + filePath
			}

			updateFile(client, variables, branchName, filePath, newContent, commitMessage)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	handleError(err)
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.image-updater.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().StringP("branch", "b", "main", "Branch for changes")
	rootCmd.Flags().StringP("label", "l", "image", "Yaml label to update the image tag value")
	rootCmd.Flags().StringP("message", "m", "", "The commit message for the tag change")
	rootCmd.Flags().IntP("nth", "n", 1, "The nth occurance of the label to update")
	rootCmd.Flags().BoolP("testLogin", "", false, "Test GITHUB_TOKEN")
	rootCmd.Flags().StringP("timestamp", "t", "", "Yaml label to update the timestamp tag value")
	rootCmd.Flags().IntP("tnth", "", 1, "The nth occurance of the timestamp label to update")
}

func getClient(githubToken string) *githubv4.Client {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	client := githubv4.NewClient(httpClient)

	return client
}

func getLoginTest(client *githubv4.Client) {
	var query struct {
		Viewer struct {
			Login     githubv4.String
			CreatedAt githubv4.DateTime
		}
	}

	err := client.Query(context.Background(), &query, nil)
	handleError(err)

	fmt.Println("    Login:", query.Viewer.Login)
	fmt.Println("CreatedAt:", query.Viewer.CreatedAt)
}

func getFileContent(client *githubv4.Client, variables map[string]interface{}) string {

	var q struct {
		Repository struct {
			File struct {
				Blob struct {
					Text     githubv4.String
					ByteSize githubv4.Int
				} `graphql:"... on Blob"`
			} `graphql:"object(expression: $filePath)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	err := client.Query(context.Background(), &q, variables)
	handleError(err)
	return string(q.Repository.File.Blob.Text)
}

func replaceTag(content string, label string, newValue string, nth int) string {
	re := regexp.MustCompile(label + ":\\s?(\\S+)")
	check := re.FindAllString(content, -1)
	if len(check) < nth {
		fmt.Fprintf(os.Stderr, "Error: File does not contain nth:%d occurances of label.", nth)
		os.Exit(1)
	}

	counter := 0
	output := re.ReplaceAllStringFunc(content, func(value string) string {
		if counter == nth {
			return value
		}

		submatches := re.FindStringSubmatch(value)
		subvalue := submatches[1]
		if strings.Contains(subvalue, ":") {
			subvalue = strings.Split(subvalue, ":")[1]
		}

		counter++
		return strings.Replace(value, subvalue, newValue, 1)
	})

	return output
}

func updateFile(client *githubv4.Client, variables map[string]interface{}, branchName string, filePath string, newContent string, message string) {
	var qOid struct {
		Repository struct {
			Object struct {
				Oid githubv4.String
			} `graphql:"object(expression:\"main\")"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	err := client.Query(context.Background(), &qOid, variables)
	handleError(err)
	oid := qOid.Repository.Object.Oid

	var m struct {
		CreateCommitOnBranch struct {
			Commit struct {
				Url githubv4.String
			}
		} `graphql:"createCommitOnBranch(input: $input)"`
	}

	repoWithOwner := githubv4.String(fmt.Sprintf("%v", variables["owner"]) + "/" + fmt.Sprintf("%v", variables["name"]))
	branch := githubv4.String(branchName)
	input := githubv4.CreateCommitOnBranchInput{
		Branch: githubv4.CommittableBranch{
			RepositoryNameWithOwner: &repoWithOwner,
			BranchName:              &branch,
		},
		Message: githubv4.CommitMessage{
			Headline: githubv4.String(message),
		},
		ExpectedHeadOid: githubv4.GitObjectID(oid),
		FileChanges: &githubv4.FileChanges{
			Additions: &[]githubv4.FileAddition{{
				Path:     githubv4.String(filePath),
				Contents: githubv4.Base64String(b64.StdEncoding.EncodeToString([]byte(newContent))),
			}},
		},
	}

	err = client.Mutate(context.Background(), &m, input, nil)
	handleError(err)
}

func handleError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", err)
		os.Exit(1)
	}
}
