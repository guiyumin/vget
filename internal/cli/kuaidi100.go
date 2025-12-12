package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/tracker"
	"github.com/spf13/cobra"
)

var (
	kuaidi100Courier string // --courier flag for courier code
)

var kuaidi100Cmd = &cobra.Command{
	Use:   "kuaidi100 <tracking_number>",
	Short: "Track package via kuaidi100 API",
	Long: `Track package delivery status using kuaidi100 API.

Examples:
  vget kuaidi100 73123456789              # Auto-detect courier
  vget kuaidi100 73123456789 -c yt        # Track YTO Express package
  vget kuaidi100 SF1234567890 -c sf       # Track SF Express package

Supported courier codes:
  sf       - 顺丰速运 (SF Express)
  yt       - 圆通速递 (YTO Express)
  sto      - 申通快递 (STO Express)
  zto      - 中通快递 (ZTO Express)
  yd       - 韵达快递 (Yunda Express)
  jt       - 极兔速递 (JiTu Express)
  jd       - 京东物流 (JD Logistics)
  ems      - EMS
  yzgn     - 邮政国内 (China Post)
  dbwl     - 德邦物流 (Deppon)
  anneng   - 安能物流 (Anneng)
  best     - 百世快递 (Best Express)
  kuayue   - 跨越速运 (Kuayue)

Configuration:
  Set your kuaidi100 API credentials:
  vget config set express.kuaidi100.key <your_key>
  vget config set express.kuaidi100.customer <your_customer_id>

  Get credentials at: https://api.kuaidi100.com/manager/v2/myinfo/enterprise`,
	Args: cobra.ExactArgs(1),
	RunE: runKuaidi100,
}

func init() {
	kuaidi100Cmd.Flags().StringVarP(&kuaidi100Courier, "courier", "c", "auto", "Courier company code (e.g., sf, yt, zto, or auto for auto-detect)")
	rootCmd.AddCommand(kuaidi100Cmd)
}

func runKuaidi100(cmd *cobra.Command, args []string) error {
	trackingNumber := args[0]

	// Load config
	cfg := config.LoadOrDefault()

	// Get kuaidi100 credentials from express config
	expressCfg := cfg.GetExpressConfig("kuaidi100")
	if expressCfg == nil || expressCfg["key"] == "" || expressCfg["customer"] == "" {
		fmt.Fprintln(os.Stderr, color.RedString("Error: kuaidi100 API credentials not configured"))
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Please set your credentials:")
		fmt.Fprintln(os.Stderr, "  vget config set express.kuaidi100.key <your_key>")
		fmt.Fprintln(os.Stderr, "  vget config set express.kuaidi100.customer <your_customer_id>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Get your credentials at: https://api.kuaidi100.com/manager/v2/myinfo/enterprise")
		return fmt.Errorf("missing kuaidi100 credentials")
	}

	// Create tracker
	t := tracker.NewKuaidi100Tracker(expressCfg["key"], expressCfg["customer"])

	// Convert courier alias to kuaidi100 code
	courierCode := tracker.GetCourierCode(kuaidi100Courier)

	// Get courier info for display
	courierInfo := tracker.GetCourierInfo(kuaidi100Courier)
	if courierInfo != nil {
		fmt.Printf("Courier: %s (%s)\n", courierInfo.Name, courierCode)
	} else if kuaidi100Courier != "auto" {
		fmt.Printf("Courier: %s\n", courierCode)
	}
	fmt.Printf("Tracking: %s\n\n", trackingNumber)

	// Track the package
	result, err := t.Track(courierCode, trackingNumber)
	if err != nil {
		return fmt.Errorf("tracking failed: %w", err)
	}

	// Display results
	printKuaidi100Result(result)

	return nil
}

func printKuaidi100Result(result *tracker.TrackingResponse) {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)

	// Status
	bold.Printf("Status: ")
	if result.IsDelivered() {
		green.Println(result.StateDescription() + " ✓")
	} else {
		yellow.Println(result.StateDescription())
	}

	fmt.Println()

	// Tracking events
	if len(result.Data) == 0 {
		fmt.Println("No tracking information available yet.")
		return
	}

	bold.Println("Tracking History:")
	fmt.Println(strings.Repeat("-", 60))

	for i, event := range result.Data {
		// Time
		timeStr := event.Ftime
		if timeStr == "" {
			timeStr = event.Time
		}
		cyan.Printf("[%s]", timeStr)
		fmt.Println()

		// Context/description
		fmt.Printf("  %s", event.Context)

		// Location if available
		if event.Location != "" {
			fmt.Printf(" (%s)", event.Location)
		} else if event.AreaName != "" {
			fmt.Printf(" (%s)", event.AreaName)
		}
		fmt.Println()

		// Add separator except for last item
		if i < len(result.Data)-1 {
			fmt.Println()
		}
	}

	fmt.Println(strings.Repeat("-", 60))
}
