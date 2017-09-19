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
	connect.Mode // [UNUSED:000][UNUSED:000][GUEST:000][USER:000][ROOT:000], 000:rwx, r:read, w:write, x:exec,
	Usage        string
	Handler      func(connect.Conn) (string, Handler)
}

// In is the tip for user input.
var In = console.In

// Cmds .
var Cmds = map[string]Command{
	"sign": {
		connect.ModeAll, // M & 000700 == M & 00070 == M & 00007 == access.
		"Sign in|out|up.",
		ParseCmd(map[string]Command{
			"in": {
				connect.ModeGuest,
				"Sign in.",
				func(c connect.Conn) (string, Handler) {
					return cout.Yes("appName: "), func(parsed *string, name string) (string, Handler) {
						return cout.Yes("Password: ") + "\033[8m", func(parsed *string, password string) (string, Handler) {
							// TODO login
							*parsed = ""
							c.SetMode(connect.ModeUser)
							return fmt.Sprintf("\033[28mSigned in: %s, mode: %05o\n"+In, name, c.GetMode()), nil
						}
					}
				},
			},
			"out": {
				connect.ModeReg, // M & 000700 == denied, guests who have not sign cannot sign out.
				"Sign out.",
				func(c connect.Conn) (string, Handler) {
					return cout.Yes("Confirm to logout?") + " (Yes or " + cout.Info("N") + "o|" + cout.Info("C") + "ancel) ",
						func(parsed *string, confirm string) (string, Handler) {
							index := strings.IndexAny(strings.ToLower(confirm), "nc")
							if index != -1 {
								return "User cancel: " + confirm[0:index] + cout.Info("%c", confirm[index]) + confirm[index+1:] + "\n" + In, nil
							}
							// TODO logout
							*parsed = ""
							c.SetMode(connect.ModeGuest)
							return "Signed out.\n" + In, nil
						}
				},
			},
			"up": {
				connect.ModeGuest,
				"Sign up account.",
				func(connect.Conn) (string, Handler) {
					return cout.Yes("appName: "), func(parsed *string, name string) (string, Handler) {
						return cout.Yes("Password: ") + "\033[8m", func(parsed *string, password string) (string, Handler) {
							return cout.Yes("\033[28mRepeat: ") + "\033[8m", func(parsed *string, repeat string) (string, Handler) {
								if repeat != password {
									return "\033[28mSign up failed, " + cout.Err("repeat password") + " not match\n" + In, nil
								}
								// TODO new account
								*parsed = ""
								return "\033[28mSigned up: " + name + " " + password + "\n" + In, nil
							}
						}
					}
				},
			},
		}),
	},
	"sudo": {
		connect.ModeRoot, // M & 000700 == M & 00070 == denied, M & 00007 == access.
		"Manage the cli-server.",
		ParseCmd(map[string]Command{
			"conn": {
				connect.ModeRoot,
				"Manage connected cli-clients.",
				ParseCmd(map[string]Command{
					"ps": {
						connect.ModeRr,
						"List connecting clients.",
						func(c connect.Conn) (string, Handler) {
							return connect.Status(c) + "\n" + In, nil
						},
					},
					"kill": {
						connect.ModeRx,
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

// CheckConn can be covered to check the connect after created.
var CheckConn = func(c connect.Conn) {
	host, _, err := net.SplitHostPort(c.RemoteAddr().String())
	if err != nil {
		return
	}
	if _, ok := map[string]bool{
		"some.ip.to.ban": true,
	}[host]; ok {
		c.SetMode(connect.ModeBan)
	}
}

var appName string

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
					"Usage: %s%s <%s>\n\nCommands list:\n%s\n"+In, appName, cout.Info(*parsed), cout.Info("Command"), strings.Join(usages, "\n"),
				), handler
			case "":
				*parsed = ""
				return cout.Yes("<<<\n") + In, nil
			default:
				if cmd, ok := cmds[args[0]]; ok {
					if conn.GetMode(cmd.Mode) == 0 {
						_, handler := handler(conn)
						return fmt.Sprintf(
							"\a%s: permission denied for command '%s', mode: %s, now: %s\n%s",
							appName, appName+*parsed+" "+cout.Err(args[0]),
							cout.Info("%05o", cmd.Mode),
							cout.Err("%05o", conn.GetMode()),
							In,
						), handler
					}
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
					appName, cout.Info(*parsed), cout.Info(strings.Join(linked, " ")),
				)
				return fmt.Sprintf(
					"\a%s: invalid option: '%s' for command '%s'.\n\n%s\n"+In,
					appName, cout.Err(args[0]), appName+*parsed, usage,
				), handler
			}
		}
	}
	return handler
}

// Start .
func Start(name, address string) error {
	appName = name
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
				CheckConn(c)

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
