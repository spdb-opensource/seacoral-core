package bankend

func Example_commonImageTemplate() {
	// tmpl, err := commonImageTemplate()
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	// fmt.Println(tmpl.Spec.Pod.Template.Spec.Containers)
	// fmt.Println(tmpl.Spec.Template)
	// fmt.Println(*tmpl.Spec.Pod.Template.Spec.TerminationGracePeriodSeconds,
	// 	tmpl.Spec.Pod.Template.Spec.HostIPC, tmpl.Spec.Pod.Template.Spec.ImagePullSecrets)

	// Output:
	// [{  [/bin/bash -c trap : TERM INT; sleep infinity & wait] []  [] [] [] {map[] map[]} [{scripts false /opt/app-root/scripts  <nil> } {custom-config false /opt/app-root/configmap  <nil> } {data false /mysqldata  <nil> } {log false /mysqllog  <nil> }] [] nil &Probe{Handler:Handler{Exec:&ExecAction{Command:[/root/server status],},HTTPGet:nil,TCPSocket:nil,},InitialDelaySeconds:30,TimeoutSeconds:5,PeriodSeconds:30,SuccessThreshold:0,FailureThreshold:2,} &Lifecycle{PostStart:nil,PreStop:&Handler{Exec:&ExecAction{Command:[/root/server stop],},HTTPGet:nil,TCPSocket:nil,},}    nil false false false}]
	// {true nil :0.0.0.0 /mysqllog /mysqldata /opt/app-root/configmap/mysqld.cnf  [] map[service-init-start:[/root/server init-start] service-start:[/root/server start] service-stop:[/root/server stop]]}
	// 60 true [{regcred}]
}
