// envctl is a CLI tool for injecting secrets from AWS Secrets Manager
// or 1Password into local development environments.
package main

import "github.com/sentiolabs/envctl/internal/cmd"

func main() {
	cmd.Execute()
}
