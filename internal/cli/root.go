// Package cli defines the command tree without global mutable command state.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/saurlax/autodl-cli/internal/api"
	"github.com/saurlax/autodl-cli/internal/config"
	"github.com/spf13/cobra"
)

type usageError struct{ error }

func invalid(format string, args ...any) error { return usageError{fmt.Errorf(format, args...)} }

type options struct {
	token, baseURL, configPath string
	timeout                    time.Duration
	json, dryRun               bool
}

// Execute returns 0 on success, 2 for CLI usage errors, and 1 for runtime errors.
func Execute(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, version string) int {
	root := New(version)
	root.SetArgs(args)
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	cmd, err := root.ExecuteContextC(ctx)
	if err == nil {
		return 0
	}
	fmt.Fprintln(errOut, "Error:", err)
	var usage usageError
	if errors.As(err, &usage) {
		if cmd == nil {
			cmd = root
		}
		_ = cmd.Help()
		return 2
	}
	// Cobra's lookup failures occur before our validators.
	if strings.HasPrefix(err.Error(), "unknown command") {
		_ = cmd.Help()
		return 2
	}
	return 1
}

func New(version string) *cobra.Command {
	o := &options{}
	root := group("autodl", "Manage AutoDL GPU instances from your terminal.",
		"An unofficial client for AutoDL's Common and Container Instance Pro APIs.\nUse a developer token from the AutoDL console. Pro APIs require identity verification.\nRun a command group without a subcommand to see its available operations.")
	root.Version = version
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })
	root.PersistentFlags().StringVar(&o.token, "token", "", "Developer token (overrides AUTODL_TOKEN and config)")
	root.PersistentFlags().StringVar(&o.configPath, "config", "", "Config file (default: OS user config directory/autodl/config.json)")
	root.PersistentFlags().StringVar(&o.baseURL, "base-url", "https://api.autodl.com", "API base URL; HTTPS required except loopback HTTP")
	root.PersistentFlags().DurationVar(&o.timeout, "timeout", 30*time.Second, "Timeout for each HTTP request, e.g. 30s or 2m")
	root.PersistentFlags().BoolVar(&o.json, "json", false, "Print the complete API response envelope as JSON")
	root.PersistentFlags().BoolVar(&o.dryRun, "dry-run", false, "Print method, URL and JSON body without sending a request")

	account := group("account", "Inspect account billing.", "Retrieve balance, voucher balance, and lifetime spending.")
	account.AddCommand(o.command("balance", "Show account balance and spending.", "Amounts are reported in CNY; the API stores thousandths of a yuan.", "autodl account balance", 0, func(cmd *cobra.Command, _ []string) (string, string, any, error) {
		return "POST", "/api/v1/dev/wallet/balance", map[string]any{}, nil
	}))

	instance := group("instance", "Create and manage Container Instance Pro instances.", "Manage the lifecycle of Container Instance Pro GPU instances.\nCreate and start incur platform charges. Release permanently deletes an instance.\nUse 'autodl instance COMMAND --help' for options and examples.")
	instance.AddCommand(o.listCommand("list", "List one page of Pro instances.", "/api/v1/dev/instance/pro/list"))
	for _, spec := range []struct{ use, short, long, method, path string }{
		{"get INSTANCE_ID", "Show instance details and connection credentials.", "Retrieve a snapshot, including SSH password and Jupyter token. Treat the output as sensitive.", "GET", "snapshot"},
		{"status INSTANCE_ID", "Show the current instance status.", "Retrieve the current status, such as running or shutdown.", "GET", "status"},
		{"stop INSTANCE_ID", "Power off an instance.", "Stop a Pro instance. This does not release its stored data.", "POST", "power_off"},
	} {
		instance.AddCommand(o.command(spec.use, spec.short, spec.long, "autodl instance "+strings.Fields(spec.use)[0]+" pro-76419909953e", 1, func(_ *cobra.Command, args []string) (string, string, any, error) {
			return spec.method, "/api/v1/dev/instance/pro/" + spec.path, map[string]any{"instance_uuid": args[0]}, nil
		}))
	}
	var startCommand string
	start := o.command("start INSTANCE_ID", "Power on an instance with GPUs.", "Only GPU mode is supported by the official API. A supplied start command overrides the creation-time command; failure does not shut down the instance.", "autodl instance start pro-76419909953e --start-command 'nvidia-smi'", 1, func(cmd *cobra.Command, args []string) (string, string, any, error) {
		body := map[string]any{"instance_uuid": args[0], "payload": "gpu"}
		if cmd.Flags().Changed("start-command") {
			body["start_command"] = startCommand
		}
		return "POST", "/api/v1/dev/instance/pro/power_on", body, nil
	})
	start.Flags().StringVar(&startCommand, "start-command", "", "Command to execute after boot (optional)")
	var yes bool
	release := o.command("release INSTANCE_ID", "Permanently release an instance.", "Permanently delete the instance and its data. Stop it first.\nExplicit --yes is required for a live request; --dry-run can preview without it.", "autodl instance release pro-76419909953e --yes", 1, func(_ *cobra.Command, args []string) (string, string, any, error) {
		if !yes && !o.dryRun {
			return "", "", nil, invalid("release permanently deletes data; pass --yes to proceed")
		}
		return "POST", "/api/v1/dev/instance/pro/release", map[string]any{"instance_uuid": args[0]}, nil
	})
	release.Flags().BoolVarP(&yes, "yes", "y", false, "Confirm permanent deletion")
	instance.AddCommand(start, release, o.createCommand())

	image := group("image", "Save and list private Pro images.", "Save an instance as a private image and inspect image preparation status.")
	image.AddCommand(o.listCommand("list", "List one page of private images.", "/api/v1/dev/instance/pro/image/private/list"))
	var imageName string
	save := o.command("save INSTANCE_ID --name NAME", "Save an instance as a private image.", "Save the instance system as a named private image. Use 'image list' to check when it is finished.", "autodl image save pro-76419909953e --name training-env", 1, func(_ *cobra.Command, args []string) (string, string, any, error) {
		if strings.TrimSpace(imageName) == "" {
			return "", "", nil, invalid("--name is required")
		}
		return "POST", "/api/v1/dev/instance/pro/image/save", map[string]any{"instance_uuid": args[0], "image_name": imageName}, nil
	})
	save.Flags().StringVar(&imageName, "name", "", "Name of the saved image (required)")
	_ = save.MarkFlagRequired("name")
	image.AddCommand(save)

	storage := group("storage", "Switch the mounted file storage type.", "Select exclusive NFS or ordinary file storage for a data center.")
	var dc, mode string
	mount := o.command("mount --data-center CODE --type TYPE", "Switch exclusive NFS or ordinary storage.", "Set --type exclusive to mount exclusive NFS, or --type ordinary to switch back to ordinary file storage. Use a data center code from AutoDL's API documentation.", "autodl storage mount --data-center westDC2 --type exclusive", 0, func(_ *cobra.Command, _ []string) (string, string, any, error) {
		if strings.TrimSpace(dc) == "" {
			return "", "", nil, invalid("--data-center is required")
		}
		n := 1
		switch mode {
		case "exclusive":
		case "ordinary":
			n = -1
		default:
			return "", "", nil, invalid("--type must be exclusive or ordinary")
		}
		return "POST", "/api/v1/dev/exclusive_nfs/mount", map[string]any{"data_center": dc, "mountable": n}, nil
	})
	mount.Flags().StringVar(&dc, "data-center", "", "AutoDL data center code (required)")
	mount.Flags().StringVar(&mode, "type", "", "Storage type: exclusive or ordinary (required)")
	_ = mount.MarkFlagRequired("data-center")
	_ = mount.MarkFlagRequired("type")
	storage.AddCommand(mount)
	root.AddCommand(account, instance, image, storage, o.configCommand())
	return root
}

func group(use, short, long string) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Long: long, Args: exactArgs(0), RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}
}

func exactArgs(n int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != n {
			return invalid("expected %d argument(s), received %d", n, len(args))
		}
		for _, arg := range args {
			if strings.TrimSpace(arg) == "" {
				return invalid("arguments must not be empty")
			}
		}
		return nil
	}
}

type requestFunc func(*cobra.Command, []string) (string, string, any, error)

func (o *options) command(use, short, long, example string, n int, request requestFunc) *cobra.Command {
	cmd := &cobra.Command{Use: use, Short: short, Long: long, Example: example, Args: exactArgs(n)}
	// Perform all usage validation before loading credentials or contacting the API.
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if err := cmd.ValidateRequiredFlags(); err != nil {
			return usageError{err}
		}
		return nil
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		method, path, body, err := request(cmd, args)
		if err != nil {
			return err
		}
		if err = o.validate(); err != nil {
			return err
		}
		if o.dryRun {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"method": method, "url": strings.TrimRight(o.baseURL, "/") + path, "body": body})
		}
		token, err := o.resolveToken()
		if err != nil {
			return err
		}
		resp, err := api.New(o.baseURL, token, o.timeout).Call(cmd.Context(), method, path, body)
		if err != nil {
			return err
		}
		return o.render(cmd.OutOrStdout(), path, resp)
	}
	return cmd
}

func (o *options) validate() error {
	if o.timeout <= 0 {
		return invalid("--timeout must be positive")
	}
	u, err := url.Parse(o.baseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return invalid("--base-url must be an absolute API origin without credentials, path, query or fragment")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")) {
		return invalid("--base-url requires HTTPS (HTTP is allowed only for loopback testing)")
	}
	return nil
}

func (o *options) path() (string, error) {
	if o.configPath != "" {
		return o.configPath, nil
	}
	return config.Path()
}

func (o *options) resolveToken() (string, error) {
	if strings.TrimSpace(o.token) != "" {
		return strings.TrimSpace(o.token), nil
	}
	if t := strings.TrimSpace(os.Getenv("AUTODL_TOKEN")); t != "" {
		return t, nil
	}
	path, err := o.path()
	if err != nil {
		return "", err
	}
	token, err := config.Token(path)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("no developer token; set AUTODL_TOKEN, pass --token, or run 'autodl config set-token'")
	}
	return token, nil
}

func (o *options) configCommand() *cobra.Command {
	cmd := group("config", "Manage local credentials.", "Store a developer token in the OS user config directory or a path selected by --config.\nToken precedence: --token, AUTODL_TOKEN, then config file.")
	set := &cobra.Command{Use: "set-token", Short: "Read and save a developer token from standard input.", Long: "Read a developer token from standard input until EOF and save it locally.\nOn Unix the new file uses mode 0600; on Windows protect it with account ACLs.\nThe token is stored as plaintext and is not validated against the API.", Example: "autodl config set-token < token.txt", Args: exactArgs(0), RunE: func(cmd *cobra.Command, _ []string) error {
		if o.dryRun {
			return invalid("--dry-run is only supported for API commands")
		}
		b, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 16385))
		if err != nil {
			return err
		}
		t := strings.TrimSpace(string(b))
		if t == "" || len(b) > 16384 || strings.ContainsAny(t, "\r\n") {
			return invalid("provide one nonempty token (maximum 16 KiB) on standard input")
		}
		path, err := o.path()
		if err != nil {
			return err
		}
		if err = config.Save(path, t); err != nil {
			return fmt.Errorf("save token: %w", err)
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Token saved to", path)
		return err
	}}
	cmd.AddCommand(set)
	return cmd
}

func (o *options) listCommand(use, short, path string) *cobra.Command {
	var page, size int
	cmd := o.command(use, short, "Retrieve a single page. Use --page to navigate; --json preserves pagination metadata.", "autodl "+map[bool]string{true: "image", false: "instance"}[strings.Contains(path, "image/")]+" list --page 1 --page-size 20", 0, func(_ *cobra.Command, _ []string) (string, string, any, error) {
		if page < 1 || size < 1 {
			return "", "", nil, invalid("--page and --page-size must be positive")
		}
		return "POST", path, map[string]any{"page_index": page, "page_size": size}, nil
	})
	cmd.Flags().IntVar(&page, "page", 1, "Page number (starting at 1)")
	cmd.Flags().IntVar(&size, "page-size", 20, "Number of records per page (positive integer)")
	return cmd
}

func (o *options) createCommand() *cobra.Command {
	var gpu, image, name, start string
	var centers []string
	var amount, disk, cuda int
	cmd := o.command("create --gpu-spec ID --image ID --cuda-min VERSION", "Create a pay-as-you-go GPU instance.", "Create and boot a Container Instance Pro GPU instance. Billing begins on the platform.\nProvide a GPU specification ID and image UUID from the official API appendix or private image list.\nCUDA minimum uses the API encoding: 113 means CUDA 11.3, 118 means 11.8.\nRegions are optional; if omitted AutoDL chooses a suitable data center.", "autodl instance create --gpu-spec pro6000-p --image base-image-l2t43iu6uk --cuda-min 118 --gpus 1 --region westDC3 --name training", 0, func(cmd *cobra.Command, _ []string) (string, string, any, error) {
		if strings.TrimSpace(gpu) == "" || strings.TrimSpace(image) == "" {
			return "", "", nil, invalid("--gpu-spec and --image must not be empty")
		}
		if amount < 1 || amount > 4 {
			return "", "", nil, invalid("--gpus must be between 1 and 4")
		}
		if disk < 0 || disk > 500 {
			return "", "", nil, invalid("--disk-gb must be between 0 and 500")
		}
		if cuda <= 0 {
			return "", "", nil, invalid("--cuda-min must be a positive encoded CUDA version, e.g. 118")
		}
		body := map[string]any{"req_gpu_amount": amount, "expand_system_disk_by_gb": disk, "gpu_spec_uuid": gpu, "image_uuid": image, "cuda_v_from": cuda}
		if len(centers) > 0 {
			for _, c := range centers {
				if strings.TrimSpace(c) == "" {
					return "", "", nil, invalid("--region values must not be empty")
				}
			}
			body["data_center_list"] = centers
		}
		if name != "" {
			body["instance_name"] = name
		}
		if cmd.Flags().Changed("start-command") {
			body["start_command"] = start
		}
		return "POST", "/api/v1/dev/instance/pro/create", body, nil
	})
	cmd.Flags().StringVar(&gpu, "gpu-spec", "", "GPU specification ID, e.g. pro6000-p (required)")
	cmd.Flags().StringVar(&image, "image", "", "Public or private image UUID (required)")
	cmd.Flags().IntVar(&cuda, "cuda-min", 0, "Minimum CUDA version encoded as an integer, e.g. 118 (required)")
	cmd.Flags().IntVar(&amount, "gpus", 1, "GPU count, 1 to 4")
	cmd.Flags().IntVar(&disk, "disk-gb", 0, "Extra system disk size in GiB, 0 to 500")
	cmd.Flags().StringSliceVar(&centers, "region", nil, "Data center codes (repeat flag or use comma-separated values)")
	cmd.Flags().StringVar(&name, "name", "", "Instance display name")
	cmd.Flags().StringVar(&start, "start-command", "", "Command to execute after boot (optional)")
	for _, flag := range []string{"gpu-spec", "image", "cuda-min"} {
		_ = cmd.MarkFlagRequired(flag)
	}
	return cmd
}

func (o *options) render(out io.Writer, path string, resp *api.Response) error {
	if o.json {
		return json.NewEncoder(out).Encode(resp)
	}
	if string(resp.Data) == "null" {
		_, err := fmt.Fprintln(out, "OK")
		return err
	}
	var scalar string
	if json.Unmarshal(resp.Data, &scalar) == nil {
		_, err := fmt.Fprintln(out, scalar)
		return err
	}
	if strings.HasSuffix(path, "/list") {
		var page struct {
			List  []map[string]any `json:"list"`
			Page  int              `json:"page_index"`
			Max   int              `json:"max_page"`
			Total int              `json:"result_total"`
		}
		if err := json.Unmarshal(resp.Data, &page); err != nil {
			return fmt.Errorf("decode list: %w", err)
		}
		tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		image := strings.Contains(path, "image/")
		if image {
			fmt.Fprintln(tw, "IMAGE ID\tNAME\tSTATUS\tSIZE (BYTES)")
		} else {
			fmt.Fprintln(tw, "INSTANCE ID\tNAME\tSTATUS\tGPU SPEC\tGPUS\tREGION")
		}
		val := func(m map[string]any, key string) string {
			if m[key] == nil {
				return "-"
			}
			return strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(fmt.Sprint(m[key]))
		}
		for _, row := range page.List {
			if image {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", val(row, "image_uuid"), val(row, "name"), val(row, "status"), val(row, "image_size"))
			} else {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", val(row, "uuid"), val(row, "name"), val(row, "status"), val(row, "gpu_spec_uuid"), val(row, "req_gpu_amount"), val(row, "region_name"))
			}
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "Page %d/%d (%d records)\n", page.Page, page.Max, page.Total)
		return err
	}
	if strings.HasSuffix(path, "/balance") {
		var b struct {
			Assets     int64 `json:"assets"`
			Accumulate int64 `json:"accumulate"`
			Voucher    int64 `json:"voucher_balance"`
		}
		if err := json.Unmarshal(resp.Data, &b); err != nil {
			return err
		}
		money := func(v int64) string {
			sign := ""
			if v < 0 {
				sign = "-"
			}
			whole := v / 1000
			fraction := v % 1000
			if whole < 0 {
				whole = -whole
			}
			if fraction < 0 {
				fraction = -fraction
			}
			return fmt.Sprintf("%s%d.%03d", sign, whole, fraction)
		}
		_, err := fmt.Fprintf(out, "Balance: %s CNY\nVouchers: %s CNY\nLifetime spend: %s CNY\n", money(b.Assets), money(b.Voucher), money(b.Accumulate))
		return err
	}
	var b bytes.Buffer
	if err := json.Indent(&b, resp.Data, "", "  "); err != nil {
		return err
	}
	_, err := fmt.Fprintln(out, b.String())
	return err
}
