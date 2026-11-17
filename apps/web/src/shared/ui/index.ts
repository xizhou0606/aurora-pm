// Icons
export { ICON_CONTEXT_SIZES, Icon, type IconComponent, type IconComponentProps, type IconProps, type IconTone } from './icons'
export type { GeneratedIconName } from './icons/icons.data'

// Primitives
export { BrandLockup, BrandMark } from './brand/brand-mark'
export { Button, buttonVariants } from './primitives/button'
export { Checkbox } from './primitives/checkbox'
export { Combobox, ComboboxClear, ComboboxContent, ComboboxEmpty, ComboboxInput, ComboboxInputGroup, ComboboxItem, ComboboxList, ComboboxTrigger, type ComboboxContentProps } from './primitives/combobox'
export { ContextMenu, ContextMenuCheckboxIndicator, ContextMenuCheckboxItem, ContextMenuContent, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger } from './primitives/context-menu'
export { ResourceContextMenu, type ResourceMenuItem } from './navigation/resource-context-menu'
export { Input } from './primitives/input'
export { Label } from './primitives/label'
export { Popover, PopoverClose, PopoverContent, PopoverDescription, PopoverTitle, PopoverTrigger } from './primitives/popover'
export { Select, SelectContent, SelectItem, SelectLabel, SelectTrigger, SelectValue, type SelectContentProps } from './primitives/select'
export { Skeleton } from './primitives/skeleton'
export { Textarea } from './primitives/textarea'

// Layout
export { OperationalPage, type OperationalPageProps, type OperationalRefreshState } from './layout/operational-page'
export { PageHeader, type PageHeaderProps } from './layout/page-header'
export { ResponsivePanel, type ResponsivePanelProps } from './layout/responsive-panel'

// Data Display
export { DataTable, DataTableSortButton, type DataTableProps, type DataTableState } from './data-display/data-table'
export { MetadataItem, MetadataList } from './data-display/metadata-list'
export { MetadataRail, type MetadataRailProps } from './data-display/metadata-rail'
export { MetricStrip, type MetricStripItem } from './data-display/metric-strip'
export { MultiSelectSummary, type MultiSelectSummaryProps } from './data-display/multi-select-summary'
export { Pagination, type PaginationProps } from './data-display/pagination'
export { PreferenceFieldList } from './data-display/preference-field-list'
export { StatusPill, type StatusPillProps, type StatusPillTone } from './data-display/status-pill'
export { Table, TableBody, TableCaption, TableCell, TableFooter, TableHead, TableHeader, TableRow } from './data-display/table'
export { TableActionButton } from './data-display/table-action-button'
export { TableColumnSettings } from './data-display/table-column-settings'
export { ViewTabs, type ViewTab } from './data-display/view-tabs'

// Feedback
export { AiLoading, AiLoadingOverlay, type AiLoadingProps, type AiLoadingOverlayProps } from './feedback/ai-loading'
export { AlertBanner, type AlertBannerProps, type AlertBannerTone } from './feedback/alert-banner'
export { BulkResult, type BulkFailure } from './misc/bulk-result'
export { ConfirmDialog, type ConfirmDialogProps } from './feedback/confirm-dialog'
export { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from './feedback/empty'
export { EmptyState, type EmptyStateProps } from './feedback/empty-state'
export { LoadingSkeleton, type LoadingSkeletonProps } from './feedback/loading-skeleton'
export { LoadingOverlay, type LoadingOverlayProps } from './feedback/loading-overlay'
export { notifyOperationError, notifyOperationSuccess, operationErrorDescription, operationFeedback, operationValidationError, OperationFeedbackProvider, type OperationFeedback, type OperationFeedbackMeta } from './feedback/operation-feedback'

// User
export { MultiUserSelect, type MultiUserSelectProps } from './user/multi-user-select'
export { UserDirectoryMultiSelect, UserDirectorySelect, type UserDirectoryItem, type UserDirectoryMultiSelectProps, type UserDirectorySelectProps, type UserDirectoryState } from './user/user-directory-select'
export { UserFaceStack, type UserFace, type UserFaceStackProps } from './user/user-face-stack'
export { UserIdentity, type UserIdentityProps } from './user/user-identity'
export { UserPreview, type UserPreviewProps } from './user/user-preview'
export { UserSelect, type UserSelectOption, type UserSelectProps } from './user/user-select'

// File & Media
export { EChartSummaryDetails, LazyEChart, type EChartSummary, type LazyEChartProps } from './file-media/lazy-echart'
export { LazyFileUploader, type LazyFileUploaderProps, type UploadFileItem } from './file-media/lazy-file-uploader'
export { LazyMediaPreview, type LazyMediaPreviewProps } from './file-media/lazy-media-preview'
export {
  LazyRichTextField,
  type LazyRichTextFieldProps,
  type RichTextDocumentLabels,
  type RichTextCollaborationAdapter,
  type RichTextDocumentSchemaVersion,
  type RichTextImageDisplay,
  type RichTextMention,
  type RichTextPastedImage,
  type RichTextPreset,
  type RichTextSelectionSnapshot,
} from './file-media/lazy-rich-text-field'
export { UploadFileCard, uploadPreviewKind, type UploadPreviewItem } from './file-media/upload-file-card'
export type { RichTextSerialization } from './edit/rich-text-serialization'
export { isRichTextEmpty, richTextVisibleCharacterCount } from './edit/rich-text-serialization'

// Theme
export { appearanceStore, bindAppearanceMedia, hydrateAppearance, readThemePaint, useAppearance, withAlpha, type ResolvedAppearance } from './theme/appearance'
export { appearanceMenuLabels, ThemeAppearanceControl } from './theme/theme-appearance-control'
export { ThemeAppearanceMenu, type AppearanceMode, type ThemeAppearanceLabels } from './theme/theme-appearance-menu'

// Navigation
export { ActivityDateValue, ActivityChange, ActivityTimeline, type ActivityTimelineItem, type ActivityTimelineProps, type ActivityTimelineState } from './navigation/activity-timeline'
export { FilterBar, type FilterBarProps } from './navigation/filter-bar'
export { PMScopeCascader, PMScopeCascaderField, ScopeCascader, type PMScopeCascaderLevels, type PMScopeCascaderLabels, type PMScopeCascaderProps, type PMScopeSelectionMode, type PMScopeCascaderState, type ScopeCascaderState } from './navigation/scope-cascader'
export { SearchInput, type SearchInputProps } from './navigation/search-input'

// Edit
export { ClearControlButton } from './edit/clear-control-button'
export {
  CopyLinkButton,
  copyInternalPath,
  resolveInternalShareHref,
  type CopyLinkButtonProps,
} from './edit/copy-link-button'
export { INLINE_EDIT_CONTROL_CLASS, InlineEditFrame, InlineEditText, InlineEditTrigger, useInlineEditField } from './edit/inline-edit-field'
// Markdown preview internals (marked + DOMPurify) are lazy-loaded inside the
// field; deep-import './edit/markdown' or './edit/markdown-preview' if the
// renderer itself is ever needed.
export { MarkdownTextField, type MarkdownTextFieldProps } from './edit/markdown-text-field'

// Misc
export { DatePicker, DateTimePicker, type DatePickerProps, type DateTimePickerProps } from './misc/date-picker'
export { DetailSection, type DetailSectionProps } from './misc/detail-section'
export { EntityDetailLayout, type EntityDetailLayoutProps } from './misc/entity-detail-layout'
export { HoverPreviewText, type HoverPreviewTextProps } from './misc/hover-preview-text'
export { IconBadge, type IconBadgeProps, type IconBadgeTone } from './misc/icon-badge'
export { FeishuLoginIcon, GoogleLoginIcon, TelegramLoginIcon } from './misc/login-provider-icons'
export { useOverlayPortalContainer } from './misc/overlay-portal-container'
export { RichTextBitableProvider, type RichTextBitableIntegration } from './file-media/rich-text-bitable-context'
export { clipboardImageFiles } from './file-media/rich-text-images'
export { parseRichTextBitableReference, containsRichTextBitable, richTextBitablePath, type RichTextBitableReference } from './file-media/rich-text-bitable-reference'

export { BulkActionBar } from './data-display/bulk-action-bar'
export { PreferenceSection } from './data-display/preference-section'
export { FormField, type FormFieldProps } from './primitives/form-field'
export { OptionSelect, OptionMultiSelect, type OptionSelectOption, type OptionSelectProps, type OptionMultiSelectProps } from './primitives/option-select'
export { RadioGroup, RadioGroupItem } from './primitives/radio-group'

export { AdaptiveFilterBar, type AdaptiveFilterControl, type ActiveFilter } from './navigation/adaptive-filter-bar'
export { UserMention } from './user/user-mention'
