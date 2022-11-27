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

	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "image-updater",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		parts := strings.Split(args[0], "/")
		owner := parts[0]
		repo := parts[1]
		label, _ := cmd.Flags().GetString("label")
		filePath := strings.Join(parts[2:], "/")
		newTag := args[1]
		branchName, _ := cmd.Flags().GetString("branch")

		variables := map[string]interface{}{
			"owner":    githubv4.String(owner),
			"name":     githubv4.String(repo),
			"filePath": githubv4.String("HEAD:" + filePath),
		}

		client := getClient()

		testLogin, _ := cmd.Flags().GetBool("testLogin")
		if testLogin {
			getLoginTest(client)
		} else {
			content := getFileContent(client, variables)

			// fmt.Printf(content + "\n\n\n")

			newContent := replaceTag(content, label, newTag)

			// fmt.Println(newContent)
			delete(variables, "filePath")
			updateFile(client, variables, branchName, filePath, newContent)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.image-updater.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().StringP("label", "l", "image", "Yaml label to update the image tag value")
	rootCmd.Flags().StringP("branch", "b", "main", "Branch for changes")
	rootCmd.Flags().BoolP("testLogin", "", false, "Test GITHUB_TOKEN")
}

func getClient() *githubv4.Client {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
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
	if err != nil {
		// Handle error.
		fmt.Println("Error!! ", err)
	}
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
	if err != nil {
		// Handle error.
		fmt.Println("Error!! ", err)
	}
	// fmt.Println(q.Repository.File.Blob.Text)
	return string(q.Repository.File.Blob.Text)
}

func replaceTag(content string, label string, newValue string) string {
	re := regexp.MustCompile(label + ":\\s?(\\S+)")

	counter := 0
	output := re.ReplaceAllStringFunc(content, func(value string) string {
		if counter == 1 {
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

func updateFile(client *githubv4.Client, variables map[string]interface{}, branchName string, filePath string, newContent string) {
	var qOid struct {
		Repository struct {
			Object struct {
				Oid githubv4.String
			} `graphql:"object(expression:\"main\")"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	err := client.Query(context.Background(), &qOid, variables)
	if err != nil {
		// Handle error.
		fmt.Println("Error!! ", err)
	}
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
			Headline: githubv4.String("Updated image tag in " + filePath),
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
	if err != nil {
		// Handle error.
		fmt.Println("Error!! ", err)
	}
	// // fmt.Println(q.Repository.File.Blob.Text)
}
