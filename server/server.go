package server

import (
	"container/list"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/zhengxiaoyao0716/util/cout"

	"github.com/zhengxiaoyao0716/zcli/connect"
)

var conns *list.List

// CmdHandler .
// Each `CmdHandler` has two step, the first, give the tip for user input, and the really handler for it.
// Next, execute the really handler with user input param, return the result and a new `CmdHandler`.
// If the new `CmdHandler` is nil, the next user input will send to the top `Cmds` variable.
// [CLIENT]: ... -> tip &                       result        -> tip &
// [SERVER]:            | handler -> execute ->        & next        | handler -> ...
type CmdHandler func(parsed *string) (tip string, handler func(args string) (result string, next CmdHandler))

// Command .
type Command struct {
	Usage   string
	Handler CmdHandler
}

// Cmds .
var Cmds = map[string]Command{
	"sign": {
		"Sign in|out|up.",
		func(parsed *string) (string, func(string) (string, CmdHandler)) {
			return ParseCmd(parsed, map[string]Command{
				"in": {
					"Sign in.",
					func(*string) (string, func(string) (string, CmdHandler)) {
						return "Name: ", func(line string) (string, CmdHandler) {
							name := line
							return "", func(*string) (string, func(string) (string, CmdHandler)) {
								return "Password: \033[8m", func(line string) (string, CmdHandler) {
									password := line
									// TODO login
									return "\033[28mSigned in: " + name + " " + password, nil
								}
							}
						}
					},
				},
				"out": {
					"Sign out.",
					func(*string) (string, func(string) (string, CmdHandler)) {
						return "Confirm to logout?", func(line string) (string, CmdHandler) {
							// TODO logout
							return "Signed out.", nil
						}
					},
				},
				"up": {
					"Sign up account.",
					func(*string) (string, func(string) (string, CmdHandler)) {
						return "Name: ", func(line string) (string, CmdHandler) {
							name := line
							// TODO create account
							return "", func(*string) (string, func(string) (string, CmdHandler)) {
								return "Password: \033[8m", func(line string) (string, CmdHandler) {
									password := line
									// TODO login
									return "\033[28mSigned up: " + name + " " + password, nil
								}
							}
						}
					},
				},
			})
		},
	},
	"sudo": {
		"Manage the cli-server.",
		func(parsed *string) (string, func(string) (string, CmdHandler)) {
			return ParseCmd(parsed, map[string]Command{
				"conn": {
					"Manage connected cli-clients.",
					func(parsed *string) (string, func(string) (string, CmdHandler)) {
						var (
							tip     string
							handler func(string) (string, CmdHandler)
						)
						tip, handler = ParseCmd(parsed, map[string]Command{
							"ls": {
								"List connected cli-clients.",
								func(parsed *string) (string, func(string) (string, CmdHandler)) {
									ls := []string{
										fmt.Sprintf("--%-3s---%-25s---%-19s--", strings.Repeat("-", 3), strings.Repeat("-", 25), strings.Repeat("-", 19)),
										fmt.Sprintf("| %-3s | %-25s | %-19s |", "ID", "ADDRESS", "TIME"),
										fmt.Sprintf("| %-3s---%-25s---%-19s |", strings.Repeat("-", 3), strings.Repeat("-", 25), strings.Repeat("-", 19)),
									}
									id := 0
									for it := conns.Front(); it != nil; it = it.Next() {
										id++
										conn := it.Value.(connect.Conn)
										ls = append(ls, fmt.Sprintf("| %03d %s", id, conn.String()))
									}
									ls = append(ls, ls[0])
									return strings.Join(ls, "\n") + "\n> ", handler
								},
							},
						})
						return tip, handler
					},
				},
			})
		},
	},
}

// Name .
var Name string

// ParseCmd .
func ParseCmd(parsed *string, cmds map[string]Command) (string, func(string) (string, CmdHandler)) {
	linked := []string{}
	for arg := range cmds {
		linked = append(linked, arg)
	}
	sort.Strings(linked)

	var usages []string
	for _, arg := range linked {
		cmd := cmds[arg]
		usages = append(usages, cout.Log("    %s\t%s", cout.Info("%10s", arg), cmd.Usage))
	}
	var (
		tip     string
		handler func(line string) (string, CmdHandler)
	)
	tip = fmt.Sprintf(
		"Usage: %s%s <%s>\n(You can also enter '--help' to check details)\n> ", Name, cout.Info(*parsed), cout.Info(strings.Join(linked, " ")),
	)
	handler = func(line string) (string, CmdHandler) {
		args := strings.SplitN(strings.TrimLeftFunc(line, unicode.IsSpace), " ", 2)
		switch args[0] {
		case "--help":
			fallthrough
		case "-h":
			return "", func(parsed *string) (string, func(string) (string, CmdHandler)) {
				return fmt.Sprintf(
					"Usage: %s%s <%s>\n\nCommands list:\n%s\n> ", Name, cout.Info(*parsed), cout.Info("Command"), strings.Join(usages, "\n"),
				), handler
			}
		case "":
			return fmt.Sprint("<<<"), nil
		default:
			if cmd, ok := cmds[args[0]]; ok {
				*parsed += " " + args[0]
				if len(args) > 1 {
					_, handler := cmd.Handler(parsed)
					return handler(args[1])
				}
				return "", cmd.Handler
			}
			return fmt.Sprintf("\a%s: invalid option: '%s' for command '%s'.", Name, cout.Err(args[0]), Name+*parsed), func(parsed *string) (string, func(string) (string, CmdHandler)) {
				return tip, handler
			}
		}
	}
	return tip, handler
}

// Start .
func Start(name, address string) error {
	Name = name
	network := "tcp"
	if strings.HasSuffix(address, ".sock") {
		network = "unix"
	}
	l, err := net.Listen(network, address)
	if err != nil {
		return err
	}

	conns = list.New()
	go func(l net.Listener) {
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Println(err)
				continue
			}

			go func(c connect.Conn) {
				ele := conns.PushFront(c)
				defer func() {
					c.Close()
					conns.Remove(ele)
				}()

				var (
					parsed  string
					handler func(string) (string, CmdHandler)
				)

				if err := c.Server(map[string]func(string) error{
					"/sys/close": func(string) error {
						return connect.ErrClose
					},
					"/sys/buf/size/sync": func(string) error {
						c.Send("/sys/buf/size/sync", strconv.Itoa(connect.BufSize))
						return nil
					},
					"/sys/ping": func(data string) error {
						fmt.Println(data)
						c.Send("/pong", "pong")
						return nil
					},
					"/usr/cmd": func(data string) error {
						if handler == nil {
							parsed = ""
							_, handler = ParseCmd(&parsed, Cmds)
						}
						result, cmdHandler := handler(data)
						var tip string
						if cmdHandler == nil {
							tip = "> "
							handler = nil
						} else {
							tip, handler = cmdHandler(&parsed)
						}
						if result != "" {
							tip = result + "\n" + tip
						}
						c.Send("/usr/cmd", tip)
						return nil
					},
				}); err != nil {
					log.Println(err)
				}
			}(connect.New(conn))
		}
	}(l)

	return nil
}

// Stop .
func Stop() {}
