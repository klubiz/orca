package apiserver

// registerRoutes wires every API endpoint to its handler.
func (s *Server) registerRoutes() {
	api := s.router.PathPrefix("/api/v1alpha1").Subrouter()

	// Health
	s.router.HandleFunc("/healthz", s.handleHealthz).Methods("GET")

	// Projects
	api.HandleFunc("/projects", s.handleListProjects).Methods("GET")
	api.HandleFunc("/projects/{name}", s.handleGetProject).Methods("GET")
	api.HandleFunc("/projects", s.handleCreateProject).Methods("POST")
	api.HandleFunc("/projects/{name}", s.handleUpdateProject).Methods("PUT")
	api.HandleFunc("/projects/{name}", s.handleDeleteProject).Methods("DELETE")

	// AgentPods - scoped by project query param: ?project=xxx
	api.HandleFunc("/agentpods", s.handleListAgentPods).Methods("GET")
	api.HandleFunc("/agentpods/{name}", s.handleGetAgentPod).Methods("GET")
	api.HandleFunc("/agentpods", s.handleCreateAgentPod).Methods("POST")
	api.HandleFunc("/agentpods/{name}", s.handleUpdateAgentPod).Methods("PUT")
	api.HandleFunc("/agentpods/{name}", s.handleDeleteAgentPod).Methods("DELETE")

	// AgentPools
	api.HandleFunc("/agentpools", s.handleListAgentPools).Methods("GET")
	api.HandleFunc("/agentpools/{name}", s.handleGetAgentPool).Methods("GET")
	api.HandleFunc("/agentpools", s.handleCreateAgentPool).Methods("POST")
	api.HandleFunc("/agentpools/{name}", s.handleUpdateAgentPool).Methods("PUT")
	api.HandleFunc("/agentpools/{name}", s.handleDeleteAgentPool).Methods("DELETE")
	api.HandleFunc("/agentpools/{name}/scale", s.handleScaleAgentPool).Methods("PUT")

	// DevTasks
	api.HandleFunc("/devtasks", s.handleListDevTasks).Methods("GET")
	api.HandleFunc("/devtasks/{name}", s.handleGetDevTask).Methods("GET")
	api.HandleFunc("/devtasks", s.handleCreateDevTask).Methods("POST")
	api.HandleFunc("/devtasks/{name}", s.handleUpdateDevTask).Methods("PUT")
	api.HandleFunc("/devtasks/{name}", s.handleDeleteDevTask).Methods("DELETE")

	// Logs
	api.HandleFunc("/agentpods/{name}/logs", s.handleGetLogs).Methods("GET")

	// Apply (generic resource creation/update)
	api.HandleFunc("/apply", s.handleApply).Methods("POST")
}
