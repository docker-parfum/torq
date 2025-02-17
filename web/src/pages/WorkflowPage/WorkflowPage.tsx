import { useState } from "react";
import useTranslations from "services/i18n/useTranslations";
import styles from "./workflow_page.module.scss";
import PageTitle from "features/templates/PageTitle";
import { Link, useParams } from "react-router-dom";
import { WORKFLOWS, MANAGE } from "constants/routes";
import { useWorkflowControls, useWorkflowData } from "./workflowHooks";
import { useUpdateWorkflowMutation } from "./workflowApi";
import WorkflowSidebar from "components/workflow/sidebar/WorkflowSidebar";
import { WorkflowCanvases } from "components/workflow/canvas/WorkflowCanvasStages";
import { StageSelector } from "components/workflow/stages/WorkflowStageSelector";

function WorkflowPage() {
  const { t } = useTranslations();

  // Fetch the workflow data
  const { workflowId, version } = useParams();
  const { workflow, workflowVersion, stageNumbers } = useWorkflowData(workflowId, version);

  const [selectedStage, setSelectedStage] = useState<number>(1);

  // construct the sidebar
  const [sidebarExpanded, setSidebarExpanded] = useState<boolean>(false);
  const workflowControls = useWorkflowControls(sidebarExpanded, setSidebarExpanded);

  const [updateWorkflow] = useUpdateWorkflowMutation();

  function handleWorkflowNameChange(name: string) {
    updateWorkflow({ workflowId: parseInt(workflowId || "0"), name: name });
  }

  const breadcrumbs = [
    <Link to={`/${MANAGE}/${WORKFLOWS}`} key={"workflowsLink"}>
      {t.workflows}
    </Link>,
    workflow?.name,
    workflowVersion?.name,
  ];

  return (
    <div className={styles.contentWrapper}>
      <PageTitle breadcrumbs={breadcrumbs} title={workflow?.name || ""} onNameChange={handleWorkflowNameChange} />
      {workflowControls}
      <div className={styles.tableWrapper}>
        <div className={styles.tableContainer}>
          <div className={styles.tableExpander}>
            <WorkflowCanvases
              selectedStage={selectedStage}
              workflowVersionId={workflowVersion?.workflowVersionId || 0}
              workflowId={workflow?.workflowId || 0}
              version={workflowVersion?.version || 0}
            />
            <StageSelector
              stageNumbers={stageNumbers}
              selectedStage={selectedStage}
              setSelectedStage={setSelectedStage}
              workflowVersionId={workflowVersion?.workflowVersionId || 0}
              workflowId={workflow?.workflowId || 0}
              version={workflowVersion?.version || 0}
            />
          </div>
        </div>
      </div>
      <WorkflowSidebar expanded={sidebarExpanded} setExpanded={setSidebarExpanded} />
    </div>
  );
}

export default WorkflowPage;
