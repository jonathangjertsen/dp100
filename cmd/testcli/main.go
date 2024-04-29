package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jonathangjertsen/dp100"
)

func main() {
	dp, err := dp100.NewDP100()
	logger := log.New(os.Stderr, "DP100 | ", 0)
	if err != nil {
		logger.Fatalf("Failed connecting to DP100: %v", err)
	}
	logger.Printf("Connected")
	err = dp.Exec(dp100.BasicInfo, []byte{})
	fmt.Printf("err=%v\n", err)
}
