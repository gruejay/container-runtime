package container

import "fmt"

func Run(args []string) error {
	fmt.Println("Running container")
	return nil
}

func Stop(args []string) error {
	fmt.Println("Stopping container")
	return nil
}

func Kill(args []string) error {
	fmt.Println("Killing container")
	return nil
}
