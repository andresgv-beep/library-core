package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tc-hib/winres"
)

func main() {
	iconPath := flag.String("icon", "", "ICO file to embed")
	outputPath := flag.String("out", "", "COFF .syso output file")
	flag.Parse()
	if *iconPath == "" || *outputPath == "" {
		fmt.Fprintln(os.Stderr, "usage: iconresource -icon app.ico -out app.syso")
		os.Exit(2)
	}

	iconFile, err := os.Open(*iconPath)
	if err != nil {
		fatal(err)
	}
	defer iconFile.Close()

	icon, err := winres.LoadICO(iconFile)
	if err != nil {
		fatal(fmt.Errorf("load icon: %w", err))
	}
	resources := winres.ResourceSet{}
	if err := resources.SetIcon(winres.RT_ICON, icon); err != nil {
		fatal(fmt.Errorf("set icon: %w", err))
	}

	output, err := os.Create(*outputPath)
	if err != nil {
		fatal(err)
	}
	defer output.Close()
	if err := resources.WriteObject(output, winres.ArchAMD64); err != nil {
		fatal(fmt.Errorf("write resources: %w", err))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
