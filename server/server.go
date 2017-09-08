package server

import (
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/zhengxiaoyao0716/util/console"

	"github.com/zhengxiaoyao0716/util/cout"

	"github.com/zhengxiaoyao0716/zcli/connect"
)

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

// In is the tip for user input.
var In = console.In

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
					return cout.Yes("Name: "), func(parsed *string, name string) (string, Handler) {
						return cout.Yes("Password: ") + "\033[8m", func(parsed *string, password string) (string, Handler) {
							// TODO login
							*parsed = ""
							return "\033[28mSigned in: " + name + " " + password + "\n" + In, nil
						}
					}
				},
			},
			"out": {
				000077, // M & 000700 == denied, guests who have not sign cannot sign out.
				"Sign out.",
				func(connect.Conn) (string, Handler) {
					return cout.Yes("Confirm to logout? "), func(parsed *string, confirm string) (string, Handler) {
						// TODO logout
						*parsed = ""
						return "Signed out.\n" + In, nil
					}
				},
			},
			"up": {
				000777,
				"Sign up account.",
				func(connect.Conn) (string, Handler) {
					return cout.Yes("Name: "), func(parsed *string, name string) (string, Handler) {
						return cout.Yes("Password: ") + "\033[8m", func(parsed *string, password string) (string, Handler) {
							// TODO new account
							*parsed = ""
							return "\033[28mSigned up: " + name + " " + password + "\n" + In, nil
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
					"ps": {
						04,
						"List connecting clients.",
						func(connect.Conn) (string, Handler) {
							return connect.Status() + "\n" + In, nil
						},
					},
					"kill": {
						01,
						"Kill a connecting client.",
						func(connect.Conn) (string, Handler) {
							return "Ids: ", func(parsed *string, ids string) (string, Handler) {
								ms := []string{}
								for _, field := range strings.Fields(ids) {
									id, err := strconv.Atoi(field)
									if err != nil {
										ms = append(ms, "invalid type of input, field: "+cout.Err(field))
									}
									c, err := connect.Get(id)
									if err != nil {
										ms = append(ms, err.Error())
									}
									c.Send("/sys/close", "administrator kill the connect.")
									if err := c.Close(); err != nil {
										ms = append(ms, err.Error())
									}
									ms = append(ms, "success killed "+cout.Info("%d", id))
								}
								return strings.Join(ms, "\n") + "\n" + In, nil
							}
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
					"Usage: %s%s <%s>\n\nCommands list:\n%s\n"+In, Name, cout.Info(*parsed), cout.Info("Command"), strings.Join(usages, "\n"),
				), handler
			case "":
				*parsed = ""
				return cout.Yes("<<<\n") + In, nil
			default:
				if cmd, ok := cmds[args[0]]; ok {
					*parsed += " " + args[0]
					output, handler := cmd.Handler(conn)
					if len(args) > 1 {
						if handler != nil {
							return handler(parsed, args[1])
						}
						output = fmt.Sprintf("redundant arguments: %s\n%s", cout.Warn(args[1]), output)
					}
					if output == "" {
						output = cmd.Usage + "\n" + In
					}
					return output, handler
				}
				_, handler := handler(conn)
				usage := fmt.Sprintf(
					"Usage: %s%s <%s>\n(You can also enter '--help' to check details)",
					Name, cout.Info(*parsed), cout.Info(strings.Join(linked, " ")),
				)
				return fmt.Sprintf(
					"\a%s: invalid option: '%s' for command '%s'.\n\n%s\n"+In,
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

	go func(l net.Listener) {
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Println(err)
				continue
			}

			go func(c connect.Conn) {
				defer c.Close()

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
