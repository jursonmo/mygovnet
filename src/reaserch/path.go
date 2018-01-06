package main

import (
	"fmt"
	"os/exec"
)

func RunCmd(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		return string(out), err
	}

	return string(out), nil
}
func main() {
	ipPath, err := exec.LookPath("ip")
	if err != nil {
		fmt.Printf("lookpath error: ", err)
	}
	confs := "ip link delete bb"
	//err := exec.Command("sh", "-c", confs).Run()
	output, err := exec.Command("sh", "-c", confs).CombinedOutput()
	if err != nil {
		fmt.Printf("my output=%s\n", string(output))
		fmt.Printf("my open err:%s, confs = %s \n", err.Error(), confs)
		return
	}

	out, err := RunCmd(ipPath, "link", "delete", "aa")
	if err != nil {
		fmt.Printf("cmd run err: %s, cmd＝ip link delete %s\n", err.Error(), "aa")
		fmt.Println(out)
	}
}
