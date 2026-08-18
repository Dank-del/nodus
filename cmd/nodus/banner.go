package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// cpmBanner is rendered with TAAG's Isometric4 FIGlet font.
// https://patorjk.com/software/taag/#p=display&f=Isometric4&t=nodus&x=none&v=4&h=4&w=80&we=false
const cpmBanner = `      ___           ___           ___           ___           ___
     /  /\         /  /\         /  /\         /  /\         /  /\
    /  /::|       /  /::\       /  /::\       /  /:/        /  /::\
   /  /:|:|      /  /:/\:\     /  /:/\:\     /  /:/        /__/:/\:\
  /  /:/|:|__   /  /:/  \:\   /  /:/  \:\   /  /:/   __   _\_ \:\ \:\
 /__/:/ |:| /\ /__/:/ \__\:\ /__/:/ \__\:| /__/:/   / /\ /__/\ \:\ \:\
 \__\/  |:|/:/ \  \:\ /  /:/ \  \:\ /  /:/ \  \:\  / /:/ \  \:\ \:\_\/
     |  |:/:/   \  \:\  /:/   \  \:\  /:/   \  \:\/ /:/   \  \:\_\:\
     |__|::/     \  \:\/:/     \  \:\/:/     \  \:\/:/     \  \:\/:/
     /__/:/       \  \::/       \__\::/       \  \::/       \  \::/
     \__\/         \__\/            ~~         \__\/         \__\/
`

func writeBanner(out io.Writer) {
	if !supportsColor(out) {
		fmt.Fprint(out, cpmBanner)
		return
	}
	colors := []int{39, 45, 51, 87, 93, 99, 105, 99, 93, 87, 51}
	for index, line := range strings.Split(strings.TrimSuffix(cpmBanner, "\n"), "\n") {
		fmt.Fprintf(out, "\x1b[38;5;%dm%s\x1b[0m\n", colors[index], line)
	}
}

func supportsColor(out io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
