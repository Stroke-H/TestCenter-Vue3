package main

import (
	"flag"
	"fmt"
	"os"

	"testcenter-server/services"
)

func main() {
	reportID := flag.String("report-id", "", "acceptance report id to send immediately")
	reporter := flag.String("reporter", "", "platform username whose reports should be grouped")
	reportDate := flag.String("report-date", "", "report creation date in YYYY-MM-DD")
	flag.Parse()

	if *reportID == "" && (*reporter == "" || *reportDate == "") {
		fmt.Fprintln(os.Stderr, "use --report-id, or use --reporter and --report-date together")
		os.Exit(2)
	}
	services.InitConfigService()
	if *reportID == "" {
		count, err := services.SendAcceptanceTodoRemindersNowForReporterDate(*reporter, *reportDate)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("acceptance todo reminder sent for %d reports\n", count)
		return
	}
	if err := services.SendAcceptanceTodoReminderNowForReport(*reportID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("acceptance todo reminder sent for report %s\n", *reportID)
}
