package groups

type Groups struct {
	Groups map[string][]string `yaml:"groups"`
}

func GetGroups(s []string, groupObj *Groups) []string {
	services := groupObj.Groups

	for k, _ := range services {
		s = append(s, k)
	}

	return s
}

func GetGroupServices(args []string, groupObj *Groups) []string {
	for i, arg := range args {
		services := groupObj.Groups
		for k, l := range services {
			if arg == k {
				args[i] = args[len(args)-1]
				args = args[:len(args)-1]
				for _, ss := range l {
					args = append(args, ss)
				}
			}
		}
	}

	return args
}
