package workflows

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"net/http"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/jmoiron/sqlx"

	"github.com/lncapital/torq/pkg/commons"
	"github.com/lncapital/torq/pkg/server_errors"
)

func RegisterWorkflowRoutes(r *gin.RouterGroup, db *sqlx.DB) {

	// Workflows Crud (/api/workflows)
	r.GET("", func(c *gin.Context) { getWorkflowsHandler(c, db) })
	r.POST("", func(c *gin.Context) { createWorkflowHandler(c, db) })
	r.PUT("", func(c *gin.Context) { updateWorkflowHandler(c, db) })
	r.DELETE("/:workflowId", func(c *gin.Context) { removeWorkflowHandler(c, db) })

	// Workflow Logs
	r.GET("/logs/:workflowId", func(c *gin.Context) { getWorkflowLogsHandler(c, db) })

	wv := r.Group("/:workflowId/versions")
	// Get all versions of a workflow
	wv.GET("", func(c *gin.Context) { getWorkflowVersionsHandler(c, db) })
	// Get a workflow version
	wv.GET("/:versionId", func(c *gin.Context) { getNodesHandler(c, db) })
	// Clone a workflow version (also used to simply add a new version)
	wv.POST("/clone", func(c *gin.Context) { cloneWorkflowVersionHandler(c, db) })
	wv.PUT("", func(c *gin.Context) { updateWorkflowVersionHandler(c, db) })
	wv.DELETE("/:versionId", func(c *gin.Context) { removeWorkflowVersionHandler(c, db) })
	// Delete a workflow Stage
	wv.DELETE("/:versionId/stage/:stage", func(c *gin.Context) { deleteStageHandler(c, db) })

	// Add, update, delete nodes to a workflow version
	nodes := r.Group("/nodes")
	nodes.POST("", func(c *gin.Context) { addNodeHandler(c, db) })
	nodes.PUT("", func(c *gin.Context) { updateNodeHandler(c, db) })
	nodes.DELETE("/:nodeId", func(c *gin.Context) { removeNodeHandler(c, db) })

	// Workflow Node Logs
	nodes.GET("/:nodeId/logs", func(c *gin.Context) { getNodeLogsHandler(c, db) })

	// Add, update, delete node links
	links := r.Group("/links")
	links.POST("", func(c *gin.Context) { addNodeLinkHandler(c, db) })
	links.PUT("", func(c *gin.Context) { updateNodeLinkHandler(c, db) })
	links.DELETE("/:linkId", func(c *gin.Context) { removeNodeLinkHandler(c, db) })

}

func getWorkflowsHandler(c *gin.Context, db *sqlx.DB) {
	workflows, err := GetWorkflows(db)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, "Getting workflows.")
		return
	}
	c.JSON(http.StatusOK, workflows)
}

func createWorkflowHandler(c *gin.Context, db *sqlx.DB) {

	storedWorkflow, err := createWorkflow(db)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, "Adding workflow.")
		return
	}

	wv, err := createWorkflowVersion(db, storedWorkflow.WorkflowId)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, "Adding workflow version to new workflow.")
		return
	}

	initialNode := CreateNodeRequest{
		WorkflowVersionId: wv.WorkflowVersionId,
		Name:              "Trigger",
		Type:              commons.WorkflowTrigger,
		Stage:             1,
	}
	_, err = createNode(db, initialNode)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, "Adding initial trigger to new workflow.")
		return
	}

	c.JSON(http.StatusOK, wv)
}

func updateWorkflowHandler(c *gin.Context, db *sqlx.DB) {
	var req UpdateWorkflow
	if err := c.BindJSON(&req); err != nil {
		server_errors.SendBadRequestFromError(c, errors.Wrap(err, server_errors.JsonParseError))
		return
	}
	storedWorkflow, err := updateWorkflow(db, req)
	if err != nil {
		pqErr, ok := err.(*pq.Error)
		if !ok {
			server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Setting workflow for workflowId: %v", req.WorkflowId))
		}
		switch true {
		case pqErr.Constraint == "workflow_name_key":
			se := server_errors.SingleFieldError("name", "Name already exists.")
			se.AddServerError(err.Error())
			server_errors.SendBadRequestFieldError(c, se)
		default:
			server_errors.WrapLogAndSendServerError(c, pqErr, fmt.Sprintf("Updating workflow for workflowId: %v", req.WorkflowId))
		}

		return
	}

	c.JSON(http.StatusOK, storedWorkflow)
}

// TODO: update removeWorkflowHandler to remove a workflow and all of its versions, nodes and links.
//   At the moment it only removes the workflow and is not in use.
func removeWorkflowHandler(c *gin.Context, db *sqlx.DB) {
	workflowId, err := strconv.Atoi(c.Param("workflowId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse workflowId in the request.")
		return
	}
	count, err := removeWorkflow(db, workflowId)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Removing workflow for workflowId: %v", workflowId))
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{"message": fmt.Sprintf("Successfully deleted %v workflow(s).", count)})
}

func deleteStageHandler(c *gin.Context, db *sqlx.DB) {

	workflowId, err := strconv.Atoi(c.Param("workflowId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse workflowId in the request.")
		return
	}
	versionId, err := strconv.Atoi(c.Param("versionId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse versionId in the request.")
		return
	}

	stage, err := strconv.Atoi(c.Param("stage"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse stage in the request.")
		return
	}

	workflowVersion, err := GetWorkflowVersion(db, workflowId, versionId)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Getting workflow version for workflowId: %v version %v", workflowId, versionId))
		return
	}

	err = deleteStage(db, workflowVersion.WorkflowVersionId, stage)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Deleting stage %v for workflowVersionId: %v", stage, workflowVersion.WorkflowVersionId))
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{"message": "Successfully deleted stage."})
}

//func getWorkflowVersionHandler(c *gin.Context, db *sqlx.DB) {
//	workflowId, err := strconv.Atoi(c.Param("workflowId"))
//	if err != nil {
//		server_errors.SendBadRequest(c, "Failed to find/parse workflowId in the request.")
//		return
//	}
//	versionId, err := strconv.Atoi(c.Param("versionId"))
//	if err != nil {
//		server_errors.SendBadRequest(c, "Failed to find/parse versionId in the request.")
//		return
//	}
//
//	workflowVersion, err := GetWorkflowVersion(db, versionId, workflowId)
//	if err != nil {
//		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Getting workflow for workflowId: %v version %v", workflowId, versionId))
//		return
//	}
//	c.JSON(http.StatusOK, workflowVersion)
//}

func getWorkflowVersionsHandler(c *gin.Context, db *sqlx.DB) {
	workflowId, err := strconv.Atoi(c.Param("workflowId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse workflowId in the request.")
		return
	}
	workflowVersions, err := GetWorkflowVersions(db, workflowId)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, "Getting workflow versions.")
		return
	}
	c.JSON(http.StatusOK, workflowVersions)
}

func cloneWorkflowVersionHandler(c *gin.Context, db *sqlx.DB) {

	workflowId, err := strconv.Atoi(c.Param("workflowId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse workflowId in the request.")
		return
	}

	version, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse version in the request.")
		return
	}

	storedWorkflowVersion, err := cloneWorkflowVersion(db, workflowId, &version)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, "Adding workflow version.")
		return
	}

	c.JSON(http.StatusOK, storedWorkflowVersion)
}

func updateWorkflowVersionHandler(c *gin.Context, db *sqlx.DB) {
	var wfv WorkflowVersion
	if err := c.BindJSON(&wfv); err != nil {
		server_errors.SendBadRequestFromError(c, errors.Wrap(err, server_errors.JsonParseError))
		return
	}
	storedWorkflowVersion, err := setWorkflowVersion(db, wfv)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Setting workflow version for workflowVersionId: %v", wfv.WorkflowVersionId))
		return
	}

	c.JSON(http.StatusOK, storedWorkflowVersion)
}

func removeWorkflowVersionHandler(c *gin.Context, db *sqlx.DB) {
	workflowId, err := strconv.Atoi(c.Param("workflowId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse workflowId in the request.")
		return
	}

	versionId, err := strconv.Atoi(c.Param("versionId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse versionId in the request.")
		return
	}

	workflowVersion, err := GetWorkflowVersion(db, workflowId, versionId)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Getting workflow version for workflowId: %v version %v", workflowId, versionId))
		return
	}

	count, err := removeWorkflowVersion(db, workflowVersion.WorkflowVersionId)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Removing workflow version for workflow: %v version %v ", workflowId, versionId))
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{"message": fmt.Sprintf("Successfully deleted %v workflow version(s).", count)})
}

//func getWorkflowVersionNodeHandler(c *gin.Context, db *sqlx.DB) {
//	workflowVersionNodeId, err := strconv.Atoi(c.Param("workflowVersionNodeId"))
//	if err != nil {
//		server_errors.SendBadRequest(c, "Failed to find/parse workflowVersionNodeId in the request.")
//		return
//	}
//	workflowVersionNode, err := GetWorkflowVersionNode(db, workflowVersionNodeId)
//	if err != nil {
//		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Getting workflow version for workflowVersionNodeId: %v", workflowVersionNodeId))
//		return
//	}
//	c.JSON(http.StatusOK, workflowVersionNode)
//}

func getNodesHandler(c *gin.Context, db *sqlx.DB) {
	workflowId, err := strconv.Atoi(c.Param("workflowId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse workflowId in the request.")
		return
	}
	versionId, err := strconv.Atoi(c.Param("versionId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse versionId in the request.")
		return
	}

	workflow, err := GetWorkflow(db, workflowId)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Getting workflow for workflowId: %v", workflowId))
		return
	}

	workflowVersion, err := GetWorkflowVersion(db, workflowId, versionId)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Getting workflow version for workflowId: %v version %v", workflowId, versionId))
		return
	}

	//workflowForest, err := GetWorkflowForest(db, workflowVersion.WorkflowVersionId)
	//if err != nil {
	//	server_errors.WrapLogAndSendServerError(c, err, "Getting workflow forest.")
	//	return
	//}

	nodes, err := GetWorkflowNodes(db, workflowVersion.WorkflowVersionId, workflow.WorkflowId, workflowVersion.Version)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, "Getting workflow nodes.")
		return
	}

	// get workflow version node links
	workflowVersionNodeLinks, err := GetWorkflowVersionNodeLinks(db, workflowVersion.WorkflowVersionId)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, "Getting workflow version node links.")
		return
	}

	r := WorkflowPage{
		Workflow: workflow,
		Version:  workflowVersion,
		Nodes:    nodes,
		Links:    workflowVersionNodeLinks,
	}

	c.JSON(http.StatusOK, r)
}

func addNodeHandler(c *gin.Context, db *sqlx.DB) {
	var req CreateNodeRequest
	if err := c.BindJSON(&req); err != nil {
		server_errors.SendBadRequestFromError(c, errors.Wrap(err, server_errors.JsonParseError))
		return
	}
	storedWorkflowVersionNode, err := createNode(db, req)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, "Adding workflow version node.")
		return
	}
	c.JSON(http.StatusOK, storedWorkflowVersionNode)
}

func updateNodeHandler(c *gin.Context, db *sqlx.DB) {

	//if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
	//	err := v.RegisterValidation("validParams", ValidateParams)
	//	if err != nil {
	//		return
	//	}
	//}

	var req UpdateNodeRequest
	if err := c.BindJSON(&req); err != nil {
		server_errors.SendBadRequestFromError(c, errors.Wrap(err, server_errors.JsonParseError))
		return
	}
	// Validate the request
	resp, err := updateNode(db, req)
	if err != nil {
		pqErr, ok := err.(*pq.Error)
		if !ok {
			server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Setting workflow version node for workflowVersionNodeId: %v", req.WorkflowVersionNodeId))
		}
		switch true {
		case pqErr.Constraint == "workflow_version_node_workflow_version_id_name_key":
			se := server_errors.SingleFieldError("name", "Name already exists.")
			se.AddServerError(err.Error())
			server_errors.SendBadRequestFieldError(c, se)
		default:
			server_errors.WrapLogAndSendServerError(c, pqErr, fmt.Sprintf("Setting workflow version node for workflowVersionNodeId: %v", req.WorkflowVersionNodeId))
		}

		return
	}

	c.JSON(http.StatusOK, resp)
}

func removeNodeHandler(c *gin.Context, db *sqlx.DB) {
	workflowVersionNodeId, err := strconv.Atoi(c.Param("nodeId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse workflowVersionNodeId in the request.")
		return
	}
	count, err := deleteNode(db, workflowVersionNodeId)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Removing workflow version node for workflowVersionNodeId: %v", workflowVersionNodeId))
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{"message": fmt.Sprintf("Successfully deleted %v workflow version node(s).", count)})
}

func addNodeLinkHandler(c *gin.Context, db *sqlx.DB) {
	var wfvnl CreateWorkflowVersionNodeLinkRequest
	if err := c.BindJSON(&wfvnl); err != nil {
		server_errors.SendBadRequestFromError(c, errors.Wrap(err, server_errors.JsonParseError))
		return
	}
	storedWorkflowVersionNodeLink, err := addWorkflowVersionNodeLink(db, wfvnl)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, "Adding workflow version node link.")
		return
	}
	c.JSON(http.StatusOK, storedWorkflowVersionNodeLink)
}

func updateNodeLinkHandler(c *gin.Context, db *sqlx.DB) {
	var wfvnl WorkflowVersionNodeLink
	if err := c.BindJSON(&wfvnl); err != nil {
		server_errors.SendBadRequestFromError(c, errors.Wrap(err, server_errors.JsonParseError))
		return
	}
	storedWorkflowVersionNodeLink, err := updateWorkflowVersionNodeLink(db, wfvnl)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Setting workflow for WorkflowVersionNodeLinkId: %v", wfvnl.WorkflowVersionNodeLinkId))
		return
	}

	c.JSON(http.StatusOK, storedWorkflowVersionNodeLink)
}

func removeNodeLinkHandler(c *gin.Context, db *sqlx.DB) {
	workflowVersionNodeLinkId, err := strconv.Atoi(c.Param("linkId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse workflowVersionNodeLinkId in the request.")
		return
	}
	count, err := removeNodeLink(db, workflowVersionNodeLinkId)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Removing workflow version node link for workflowVersionNodeLinkId: %v", workflowVersionNodeLinkId))
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{"message": fmt.Sprintf("Successfully deleted %v workflow version node link(s).", count)})
}

func getWorkflowLogsHandler(c *gin.Context, db *sqlx.DB) {
	workflowId, err := strconv.Atoi(c.Param("workflowId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse workflowId in the request.")
		return
	}
	workflowLogs, err := GetWorkflowLogs(db, workflowId, commons.WORKFLOW_LOG_COUNT)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Getting workflow logs for workflowId: %v", workflowId))
		return
	}
	c.JSON(http.StatusOK, workflowLogs)
}

func getNodeLogsHandler(c *gin.Context, db *sqlx.DB) {
	workflowVersionNodeId, err := strconv.Atoi(c.Param("nodeId"))
	if err != nil {
		server_errors.SendBadRequest(c, "Failed to find/parse workflowVersionNodeId in the request.")
		return
	}
	workflowVersionNodeLogs, err := GetNodeLogs(db, workflowVersionNodeId, commons.WORKFLOW_LOG_COUNT)
	if err != nil {
		server_errors.WrapLogAndSendServerError(c, err, fmt.Sprintf("Getting workflow version node logs for workflowVersionNodeId: %v", workflowVersionNodeId))
		return
	}
	c.JSON(http.StatusOK, workflowVersionNodeLogs)
}
