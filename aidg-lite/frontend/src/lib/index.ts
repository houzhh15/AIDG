/** aidg-lite-ui -- Public library entry point */

// API clients (non-conflicting)
export * from '../api/auth';
export * from '../api/client';
export * from '../api/copy';
export * from '../api/currentTask';
export * from '../api/documents';
export * from '../api/permissions';
export * from '../api/projectDocs';
export * from '../api/projects';
export * from '../api/remotes';
export * from '../api/resourceApi';
export * from '../api/statisticsApi';
export * from '../api/taskSummaryApi';

// Namespace aliases for modules with overlapping symbol names
export * as TasksApi from '../api/tasks';
export * as TaskDocsApi from '../api/taskDocs';
export * as UsersApi from '../api/users';

// Hooks
export * from '../hooks/useLiteMode';
export * from '../hooks/usePermission';
export * from '../hooks/useProjectDeliverable';
export * from '../hooks/useProjectPermission';

// Contexts
export * from '../contexts/PermissionContext';
export * from '../contexts/TaskRefreshContext';

// Components with named exports (export const X)
export * from '../components/ArchitectureDesign';
export * from '../components/CopyResourceDialog';
export * from '../components/Deliverables';
export * from '../components/DiffModal';
export * from '../components/DocumentTOC';
export * from '../components/FeatureList';
export * from '../components/MermaidChart';
export * from '../components/ProjectSidebar';
export * from '../components/SectionContentEditor';
export * from '../components/SectionEditor';
export * from '../components/SectionTree';
export * from '../components/StepEditorModal';
export * from '../components/TaskDocIncremental';
export * from '../components/TaskLinkedDocuments';
export * from '../components/TaskSelector';
export * from '../components/TaskSummaryPanel';
export * from '../components/TechDesign';
export * from '../components/project/ProjectArchitectureDesign';
export * from '../components/project/ProjectFeatureList';
export * from '../components/project/ProjectTechDesign';
export * from '../components/TagManagement/TagButton';
export * from '../components/TagManagement/TagConfirmModal';
export * from '../components/TagManagement/TagVersionSelect';

// Components with default exports (export default X)
export { default as ContextManagerDropdown } from '../components/project/ContextManagerDropdown';
export { default as ExecutionPlanView } from '../components/ExecutionPlanView';
export { default as MarkdownViewer } from '../components/MarkdownViewer';
export { default as NoPermissionPage } from '../components/NoPermissionPage';
export { default as PlanEditor } from '../components/ExecutionPlan/PlanEditor';
export { default as ProjectDocSectionEditor } from '../components/project/ProjectDocSectionEditor';
export { default as ProjectDocument } from '../components/project/ProjectDocument';
export { default as ProjectTaskSelector } from '../components/ProjectTaskSelector';
export { default as ProjectTaskSidebar } from '../components/ProjectTaskSidebar';
export { default as RecommendationPanel } from '../components/RecommendationPanel';
export { default as ResourceEditor } from '../components/resources/ResourceEditor';
export { default as ResourceEditorModal } from '../components/resources/ResourceEditorModal';
export { default as TaskDocuments } from '../components/TaskDocuments';
// documents sub-components (direct imports to avoid type-re-export conflicts)
export { default as MarkdownEditor } from '../components/documents/MarkdownEditor';
export { default as EnhancedTreeView } from '../components/documents/EnhancedTreeView';
export { default as FileUploadArea } from '../components/documents/FileUploadArea';
// permission sub-components
export { default as NoPermission } from '../components/permission/NoPermission';
export { default as PermissionGuard } from '../components/permission/PermissionGuard';

// Services
export * from '../services/tagService';

// Types -- using namespace aliases to avoid conflicts with API module types
export * as CoreTypes from '../types';
export * as DocTypes from '../types/documents';
export * from '../types/prompt';
export * from '../types/section';
export * from '../components/resources/types';

// Utils
export * from '../utils/planMarkdownBuilder';

// Constants
export * from '../constants/permissions';
