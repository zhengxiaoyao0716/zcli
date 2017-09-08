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

// Handler .
// If the return handler is nil, the next handler would be set to:
// 1. top handler for `Cmds`, if parsed was "" too.
// 2. last handler for `parsed` value, if it not "".
type Handler func(parsed *string, input string) (string, Handler)

// Command .
type Command struct {
	Mode    int16 // [UNUSED:000][UNUSED:000][GUEST:000][USER:000][ROOT:000], 000:rwx, r:read, w:write, x:exec,
	Usage   string
	Handler func(connect.Conn) (string, Handler)
}

// Cmds .
var Cmds = map[string]Command{
	"sign": {
		000777, // M & 000700 == M & 00070 == M & 00007 == access.
		"Sign in|out|up.",
		ParseCmd(map[string]Command{
			"in": {
				000777,
				"Sign in.",
				func(connect.Conn) (string, Handler) {
					return "Name: ", func(parsed *string, name string) (string, Handler) {
						return "Password: \033[8m", func(parsed *string, password string) (string, Handler) {
							// TODO login
							*parsed = ""
							return "\033[28mSigned in: " + name + " " + password + "\n> ", nil
						}
					}
				},
			},
			"out": {
				000077, // M & 000700 == denied, guests who have not sign cannot sign out.
				"Sign out.",
				func(connect.Conn) (string, Handler) {
					return "Confirm to logout? ", func(parsed *string, confirm string) (string, Handler) {
						// TODO logout
						*parsed = ""
						return "Signed out.\n> ", nil
					}
				},
			},
			"up": {
				000777,
				"Sign up account.",
				func(connect.Conn) (string, Handler) {
					return "Name: ", func(parsed *string, name string) (string, Handler) {
						return "Password: \033[8m", func(parsed *string, password string) (string, Handler) {
							// TODO new account
							*parsed = ""
							return "\033[28mSigned up: " + name + " " + password + "\n> ", nil
						}
					}
				},
			},
		}),
	},
	"sudo": {
		000007, // M & 000700 == M & 00070 == denied, M & 00007 == access.
		"Manage the cli-server.",
		ParseCmd(map[string]Command{
			"conn": {
				07,
				"Manage connected cli-clients.",
				ParseCmd(map[string]Command{
					"ls": {
						07,
						"List connected cli-clients.",
						func(connect.Conn) (string, Handler) {
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
							return strings.Join(ls, "\n") + "\n> ", nil
						},
					},
				}),
			},
		}),
	},
}

// Name .
var Name string

// ParseCmd .
func ParseCmd(cmds map[string]Command) func(connect.Conn) (string, Handler) {
	linked := []string{}
	for arg := range cmds {
		linked = append(linked, arg)
	}
	sort.Strings(linked)

	var handler func(conn connect.Conn) (string, Handler)
	handler = func(conn connect.Conn) (string, Handler) {
		return "", func(parsed *string, input string) (string, Handler) {
			args := strings.SplitN(strings.TrimLeftFunc(input, unicode.IsSpace), " ", 2)
			switch args[0] {
			case "--help":
				fallthrough
			case "-h":
				var usages []string
				for _, arg := range linked {
					cmd := cmds[arg]
					usages = append(usages, fmt.Sprintf("    %s\t%s", cout.Info("%10s", arg), cmd.Usage))
				}
				_, handler := handler(conn)
				return fmt.Sprintf(
					"Usage: %s%s <%s>\n\nCommands list:\n%s\n> ", Name, cout.Info(*parsed), cout.Info("Command"), strings.Join(usages, "\n"),
				), handler
			case "":
				*parsed = ""
				return fmt.Sprint("<<<\n> "), nil
			default:
				if cmd, ok := cmds[args[0]]; ok {
					*parsed += " " + args[0]
					output, handler := cmd.Handler(conn)
					if len(args) > 1 {
						return handler(parsed, args[1])
					}
					if output == "" {
						output = cmd.Usage + "\n> "
					}
					return output, handler
				}
				_, handler := handler(conn)
				usage := fmt.Sprintf(
					"Usage: %s%s <%s>\n(You can also enter '--help' to check details)",
					Name, cout.Info(*parsed), cout.Info(strings.Join(linked, " ")),
				)
				return fmt.Sprintf(
					"\a%s: invalid option: '%s' for command '%s'.\n\n%s\n> ",
					Name, cout.Err(args[0]), Name+*parsed, usage,
				), handler
			}
		}
	}
	return handler
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

				var parsed string
				_, handler := ParseCmd(Cmds)(c)

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
					"/usr/cmd": func(input string) error {
						result, next := handler(&parsed, input)
						if next == nil {
							_, next = ParseCmd(Cmds)(c)
							if parsed != "" {
								li := strings.LastIndex(parsed, " ")
								if li == -1 {
									li = 0
								}
								input = strings.TrimSpace(parsed[0:li])
								parsed = ""
								_, next = next(&parsed, input)
							}
						}
						handler = next
						c.Send("/usr/cmd", result)
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
