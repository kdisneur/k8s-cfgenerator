package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fewlinesco/k8s-cfgenerator/cmd/cfgenerator/internal"
	"github.com/fewlinesco/k8s-cfgenerator/cmd/cfgenerator/internal/file"
	"github.com/fewlinesco/k8s-cfgenerator/cmd/cfgenerator/internal/interpreter"
)

const usageFmt = `Synopsis

	%[1]s [-interpreter=plain|jsonnet] [volume-paths ...]

Description

	Reads a content (plain text or JSONNET) template and output the result
	to a file (as a JSON or plain text).

	It reads all files present in 'the volume-paths' folders and for each of
	these files, sets the file name as variable name and the content of the
	file as value.

Flags

	-in=<template-path>|-
	   A path to the template to use as input. When using "-" input is STDIN.
	   (Default: -)

	-interpreter=plain|jsonnet
	   When plain, interprets the input as plain text and use gotpl as
	   variable system.

	   When jsonnet, interprets the input as JSONNET and use extVar as
	   variable system.

	   By default it is set to jsonnet

	-out=<file>|-
	   A path to where to generate the file. When using "-" output is STDOUT.
	   (Default: -)

	   Note that you can pass the flag several times if the goal is to write
	   the configuration in several locations. It can be useful to add an
	   additional '-out=-' for debugging purpose for example.

Arguments

	[volume-paths ...]
	   a list of folder or files.

	   When file: the content of the file will be loaded and set in a JSONNET
	   extVar named with the file name.

	   When folder: the content of each of the file of the folder will be
	   loaded and set in a JSONNET extVar named with the file name.
	   The script doesn't load files in sub folders.

Examples

	1. read all files in /data/configmap and /data/secrets and use their name
	   as JSONNET extvar. Then evaluates STDIN and generate a JSON in STDOUT

	   $> %[1]s /data/configmap /data/secrets <<EOF
	   {
	   	api: {
	   		address: '0.0.0.0:' + std.extVar('API_PORT'),
	   	},
	   	database: {
	   		password: std.extVar("DATABASE_PASSWORD"),
	   		username: std.extVar("DATABASE_USERNAME"),
	   	},
	   }
	   EOF

	2. read all files in /data/configmap and /data/secrets and use their name
	   as JSONNET extvar. Then evaluates /app/config.jsonnet and generate a
	   JSON in /app/config.json

	   $> %[1]s -in /app/confg.jsonnet -out /app/config.json /data/configmap /data/secrets

`

type stringsFlag []string

func (s *stringsFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringsFlag) Set(value string) error {
	*s = append(*s, value)

	return nil
}

func main() {
	var cfg = struct {
		InterpreterName string
		In              string
		Outs            stringsFlag
	}{
		InterpreterName: "jsonnet",
		In:              "-",
	}

	flag.Usage = func() { fmt.Fprintf(flag.CommandLine.Output(), usageFmt, filepath.Base(os.Args[0])) }
	flag.StringVar(&cfg.InterpreterName, "interpreter", cfg.InterpreterName, "")
	flag.StringVar(&cfg.In, "in", cfg.In, "")
	flag.Var(&cfg.Outs, "out", "")

	flag.Parse()

	if len(cfg.Outs) == 0 {
		cfg.Outs = append(cfg.Outs, "-")
	}

	if err := run(cfg.InterpreterName, cfg.In, cfg.Outs, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(interpreterName string, inputPath string, outputPaths []string, volumes []string) error {
	runtime, found := interpreter.Get(interpreterName)
	if !found {
		return fmt.Errorf("unsupported interpreter '%s'", interpreterName)
	}

	input, err := file.OpenInput(inputPath)
	if err != nil {
		return fmt.Errorf("can't open input file '%s': %v", inputPath, err)
	}
	defer input.Close()

	content, err := internal.Generate(runtime, input, volumes)
	if err != nil {
		return fmt.Errorf("can't generate content: %v", err)
	}

	outputs := make([]*os.File, len(outputPaths))
	for i, outputPath := range outputPaths {
		output, err := file.OpenOutput(outputPath)
		if err != nil {
			return fmt.Errorf("can't open output file '%s': %v", outputPath, err)
		}
		defer output.Close()

		outputs[i] = output
	}

	for i := range outputs {
		fmt.Fprint(outputs[i], content)
	}

	return nil
}
