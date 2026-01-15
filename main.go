// envctl is a CLI tool for injecting AWS Secrets Manager secrets
// into local development environments.
package main

import "github.com/sentiolabs/envctl/internal/cmd"

func main() {
	cmd.Execute()
}
