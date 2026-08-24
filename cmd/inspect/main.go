package main

import (
	"fmt"
	"log"

	"github.com/xuri/excelize/v2"
)

func main() {
	file := "data/ParcelPilot_Assessment_Data.xlsx"

	f, err := excelize.OpenFile(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	sheets := f.GetSheetList()

	fmt.Println("Sheets:")
	for _, sheet := range sheets {
		fmt.Printf("\n=== %s ===\n", sheet)

		rows, err := f.GetRows(sheet)
		if err != nil {
			log.Fatal(err)
		}

		for i, row := range rows {
			fmt.Printf("%d: %v\n", i+1, row)

			if i >= 5 {
				break
			}
		}
	}
}
