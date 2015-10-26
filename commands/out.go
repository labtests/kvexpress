package commands

import (
	kvexpress "../kvexpress/"
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"strconv"
)

var outCmd = &cobra.Command{
	Use:   "out",
	Short: "Write a file based on key data.",
	Long:  `out is for writing a file based on a Consul key.`,
	Run:   outRun,
}

func outRun(cmd *cobra.Command, args []string) {
	checkOutFlags()

	key_data := kvexpress.KeyDataPath(KeyOutLocation, PrefixLocation)
	key_checksum := kvexpress.KeyChecksumPath(KeyOutLocation, PrefixLocation)

	// Get the KV data out of Consul.
	KVData := kvexpress.Get(key_data, ConsulServer, Token)

	// Get the Checksum data out of Consul.
	Checksum := kvexpress.Get(key_checksum, ConsulServer, Token)

	// Is the data long enough?
	longEnough := kvexpress.LengthCheck(KVData, MinFileLength)
	log.Print("out: longEnough='", strconv.FormatBool(longEnough), "'")

	// Does the checksum match?
	checksumMatch := kvexpress.ChecksumCompare(KVData, Checksum)
	log.Print("out: checksumMatch='", strconv.FormatBool(checksumMatch), "'")

	// If the data is long enough and the checksum matches, write the file.
	if longEnough && checksumMatch {
		kvexpress.WriteFile(KVData, FiletoWrite, FilePermissions)
	} else {
		log.Print("Could not write file.")
	}

	// Run this command after the file is written.
	if PostExec != "" {
		log.Print("out: exec='", PostExec, "'")
		kvexpress.RunCommand(PostExec)
	}
}

func checkOutFlags() {
	log.Print("out: Checking cli flags.")
	if KeyOutLocation == "" {
		fmt.Println("Need a key location in -k")
		os.Exit(1)
	}
	if FiletoWrite == "" {
		fmt.Println("Need a file to write in -f")
		os.Exit(1)
	}
	log.Print("out: Required cli flags present.")
}

var KeyOutLocation string
var FiletoWrite string
var MinFileLength int
var FilePermissions int

func init() {
	RootCmd.AddCommand(outCmd)
	outCmd.Flags().IntVarP(&FilePermissions, "chmod", "c", 0640, "permissions for the file")
	outCmd.Flags().StringVarP(&KeyOutLocation, "key", "k", "", "key to pull data from")
	outCmd.Flags().StringVarP(&FiletoWrite, "file", "f", "", "where to write the data")
	outCmd.Flags().IntVarP(&MinFileLength, "length", "l", 10, "minimum amount of lines in the file")
}
