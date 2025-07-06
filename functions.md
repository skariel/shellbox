./cmd/server/server.go:13:func main() {
./cmd/deploy/main.go:11:func main() {
./internal/infra/tables.go:23:func CreateTableStorageResources(ctx context.Context, clients *AzureClients, accountName string, tableNames []string) TableStorageResult {
./internal/infra/tables.go:29:func CreateTableStorageResourcesInResourceGroup(ctx context.Context, clients *AzureClients, resourceGroupName, accountName string, tableNames []string) TableStorageResult {
./internal/infra/tables.go:62:func ensureStorageAccountExists(ctx context.Context, storageClient *armstorage.AccountsClient, resourceGroupName, accountName string) error {
./internal/infra/tables.go:102:func getStorageConnectionString(ctx context.Context, storageClient *armstorage.AccountsClient, resourceGroupName, accountName string) (string, error) {
./internal/infra/tables.go:124:func createTables(ctx context.Context, connectionString string, tableNames []string) error {
./internal/infra/tables.go:175:func writeTableEntity(ctx context.Context, clients *AzureClients, tableName string, entity interface{}) error {
./internal/infra/tables.go:193:func upsertTableEntity(ctx context.Context, clients *AzureClients, tableName string, entity interface{}) error {
./internal/infra/tables.go:213:func WriteEventLog(ctx context.Context, clients *AzureClients, event *EventLogEntity) error {
./internal/infra/tables.go:220:func WriteResourceRegistry(ctx context.Context, clients *AzureClients, resource *ResourceRegistryEntity) error {
./internal/infra/retry.go:11:func RetryOperation(ctx context.Context, operation func(context.Context) error, timeout, interval time.Duration, operationName string) error {
./internal/infra/instances.go:39:func CreateInstance(ctx context.Context, clients *AzureClients, config *VMConfig) (string, error) {
./internal/infra/instances.go:81:func createInstanceNSG(ctx context.Context, clients *AzureClients, nsgName string) (*armnetwork.SecurityGroup, error) {
./internal/infra/instances.go:196:func createInstanceNIC(ctx context.Context, clients *AzureClients, nicName string, nsgID *string) (*armnetwork.Interface, error) {
./internal/infra/instances.go:233:func buildStorageProfile(config *VMConfig, instanceID string, namer *ResourceNamer) (*armcompute.StorageProfile, error) {
./internal/infra/instances.go:264:func createInstanceVM(ctx context.Context, clients *AzureClients, vmName, nicID string, config *VMConfig, tags *InstanceTags) (*armcompute.VirtualMachine, error) {
./internal/infra/instances.go:350:func GeneralizeVM(ctx context.Context, clients *AzureClients, resourceGroupName, vmName string) error {
./internal/infra/instances.go:411:func GenerateInstanceInitScript() (string, error) {
./internal/infra/instances.go:437:func DeleteInstance(ctx context.Context, clients *AzureClients, resourceGroupName, vmName string) error {
./internal/infra/instances.go:461:func extractInstanceResourceInfo(vm *armcompute.VirtualMachinesClientGetResponse, vmName, resourceGroupName string, vmExists bool) instanceResourceInfo {
./internal/infra/instances.go:480:func extractResourcesFromVM(info *instanceResourceInfo, vm *armcompute.VirtualMachinesClientGetResponse) {
./internal/infra/instances.go:503:func ExtractInstanceIDFromVMName(vmName string) string {
./internal/infra/instances.go:512:func generateMissingResourceNames(info *instanceResourceInfo, resourceGroupName string) {
./internal/infra/instances.go:530:func DeleteVM(ctx context.Context, clients *AzureClients, resourceGroupName, vmName string, vmExists bool) {
./internal/infra/instances.go:551:func DeleteDisk(ctx context.Context, clients *AzureClients, resourceGroupName, diskName, diskType string) {
./internal/infra/instances.go:572:func DeleteNIC(ctx context.Context, clients *AzureClients, resourceGroupName, nicName, nicID string) {
./internal/infra/instances.go:599:func DeleteNSG(ctx context.Context, clients *AzureClients, resourceGroupName, nsgName string) {
./internal/infra/instances.go:620:func UpdateInstanceStatus(ctx context.Context, clients *AzureClients, instanceID, status string) error {
./internal/infra/instances.go:663:func UpdateInstanceStatusAndUser(ctx context.Context, clients *AzureClients, instanceID, status, userID string) error {
./internal/infra/instances.go:708:func GetInstancePrivateIP(ctx context.Context, clients *AzureClients, instanceID string) (string, error) {
./internal/infra/instances.go:730:func AttachVolumeToInstance(ctx context.Context, clients *AzureClients, instanceID, volumeID string) error {
./internal/infra/instances.go:777:func DetachVolumeFromInstance(ctx context.Context, clients *AzureClients, instanceID, volumeID string) error {
./internal/infra/instances.go:827:func waitForInstanceInResourceGraph(ctx context.Context, clients *AzureClients, instanceID string, expectedTags *InstanceTags) error {
./internal/infra/golden_snapshot.go:29:func GenerateQEMUInitScript(config QEMUScriptConfig) (string, error) {
./internal/infra/golden_snapshot.go:197:func CreateGoldenSnapshotIfNotExists(ctx context.Context, clients *AzureClients) (*GoldenSnapshotInfo, error) {
./internal/infra/golden_snapshot.go:301:func createAndProvisionBoxWithDataVolume(ctx context.Context, clients *AzureClients, resourceGroupName, vmName, sshPublicKey string) (*tempBoxInfo, error) {
./internal/infra/golden_snapshot.go:387:func waitForQEMUReady(ctx context.Context, _ *AzureClients, tempBox *tempBoxInfo) error {
./internal/infra/golden_snapshot.go:550:func createDataSnapshotAndOSImage(ctx context.Context, clients *AzureClients, resourceGroupName, dataSnapshotName, osSnapshotName string, tempBox *tempBoxInfo) (*GoldenSnapshotInfo, error) {
./internal/infra/golden_snapshot.go:626:func createBoxVMWithDataDisk(ctx context.Context, clients *AzureClients, resourceGroupName, vmName, nicID, dataDiskID, sshPublicKey string) (*armcompute.VirtualMachine, error) {
./internal/infra/golden_snapshot.go:709:func generateDataVolumeInitScript(_ context.Context, _ *AzureClients, sshPublicKey string) (string, error) {
./internal/infra/golden_snapshot.go:728:func ExtractDiskNameFromID(diskID string) string {
./internal/infra/golden_snapshot.go:734:func ExtractSuffix(resourceGroupName string) string {
./internal/infra/golden_snapshot.go:744:func ensureGoldenSnapshotResourceGroup(ctx context.Context, clients *AzureClients) error {
./internal/infra/golden_snapshot.go:774:func generateGoldenSnapshotNames(sshPublicKey string) (dataSnapshotName, imageName string, err error) {
./internal/infra/resource_graph_wait.go:11:func waitForVolumeInResourceGraph(ctx context.Context, clients *AzureClients, volumeID string, expectedTags *VolumeTags) error {
./internal/infra/resource_graph_wait.go:54:func waitForVolumeTagsInResourceGraph(ctx context.Context, clients *AzureClients, volumeID string, expectedTags map[string]string) error {
./internal/infra/resource_graph_wait.go:102:func waitForInstanceTagsInResourceGraph(ctx context.Context, clients *AzureClients, instanceID string, expectedTags map[string]string) error {
./internal/infra/clients.go:23:func FatalOnError(err error, message string) {
./internal/infra/clients.go:30:func createAzureClients(clients *AzureClients) {
./internal/infra/clients.go:73:func createTableClient(clients *AzureClients) {
./internal/infra/clients.go:96:func waitForRoleAssignment(ctx context.Context, cred azcore.TokenCredential) string {
./internal/infra/clients.go:119:func readTableStorageConfig(clients *AzureClients) error {
./internal/infra/clients.go:138:func NewAzureClients(suffix string, useAzureCli bool) *AzureClients {
./internal/infra/pool.go:55:func NewDevPoolConfig() PoolConfig {
./internal/infra/pool.go:78:func NewBoxPool(clients *AzureClients, vmConfig *VMConfig, poolConfig PoolConfig, goldenSnapshot *GoldenSnapshotInfo) *BoxPool {
./internal/infra/pool.go:94:func (p *BoxPool) MaintainPool(ctx context.Context) {
./internal/infra/pool.go:110:func (p *BoxPool) maintainInstancePool(ctx context.Context) {
./internal/infra/pool.go:129:func (p *BoxPool) maintainVolumePool(ctx context.Context) {
./internal/infra/pool.go:148:func (p *BoxPool) scaleUpInstances(ctx context.Context, currentSize int) {
./internal/infra/pool.go:202:func (p *BoxPool) scaleDownInstances(ctx context.Context, currentSize int) {
./internal/infra/pool.go:265:func (p *BoxPool) scaleUpVolumes(ctx context.Context, currentSize int) {
./internal/infra/pool.go:328:func (p *BoxPool) scaleDownVolumes(ctx context.Context, currentSize int) {
./internal/infra/qmp_helpers.go:49:func executeQMPCommands(ctx context.Context, commands []string, instanceIP string) ([]QMPResponse, error) {
./internal/infra/qmp_helpers.go:83:func parseQMPResponses(output string) ([]QMPResponse, error) {
./internal/infra/qmp_helpers.go:115:func checkQMPSuccess(responses []QMPResponse) error {
./internal/infra/qmp_helpers.go:160:func GetMigrationInfo(ctx context.Context, instanceIP string) (*MigrationInfo, error) {
./internal/infra/qmp_helpers.go:193:func calculateCheckInterval(progress float64) time.Duration {
./internal/infra/qmp_helpers.go:207:func handleActiveMigration(info *MigrationInfo, progress *migrationProgress, startTime time.Time) time.Duration {
./internal/infra/qmp_helpers.go:258:func ExecuteMigrationCommand(ctx context.Context, instanceIP, stateFile string) error {
./internal/infra/qmp_helpers.go:308:func SendKeyCommand(ctx context.Context, keys []string, instanceIP string) error {
./internal/infra/network.go:22:func createNSGRule(name, protocol, srcAddr, dstAddr, dstPort string, access armnetwork.SecurityRuleAccess, priority int32, direction armnetwork.SecurityRuleDirection) *armnetwork.SecurityRule {
./internal/infra/network.go:73:func createResourceGroup(ctx context.Context, clients *AzureClients) {
./internal/infra/network.go:86:func createBastionNSG(ctx context.Context, clients *AzureClients) {
./internal/infra/network.go:102:func createVirtualNetwork(ctx context.Context, clients *AzureClients) {
./internal/infra/network.go:142:func setSubnetIDsFromVNet(clients *AzureClients, vnetResult armnetwork.VirtualNetworksClientCreateOrUpdateResponse) {
./internal/infra/network.go:160:func InitializeTableStorage(clients *AzureClients, useAzureCli bool) {
./internal/infra/network.go:186:func CreateNetworkInfrastructure(ctx context.Context, clients *AzureClients, useAzureCli bool) {
./internal/infra/constants.go:159:func FormatConfig(suffix string) string {
./internal/infra/constants.go:175:func formatNSGRules(rules []*armnetwork.SecurityRule) string {
./internal/infra/constants.go:188:func GenerateConfigHash(suffix string) (string, error) {
./internal/infra/qemu_manager.go:18:func NewQEMUManager(clients *AzureClients) *QEMUManager {
./internal/infra/qemu_manager.go:25:func (qm *QEMUManager) StartQEMUWithVolume(ctx context.Context, instanceIP, _ string) error {
./internal/infra/qemu_manager.go:209:func (qm *QEMUManager) StopQEMU(ctx context.Context, instanceIP string) error {
./internal/infra/qemu_manager.go:228:func (qm *QEMUManager) SendGuestExecCommand(ctx context.Context, instanceIP, command string, args []string) error {
./internal/infra/logger.go:9:func NewLogger() *slog.Logger {
./internal/infra/logger.go:20:func SetDefaultLogger() {
./internal/infra/volumes.go:44:func CreateVolume(ctx context.Context, clients *AzureClients, config *VolumeConfig) (string, error) {
./internal/infra/volumes.go:69:func CreateVolumeWithTags(ctx context.Context, clients *AzureClients, resourceGroupName, volumeName string, sizeGB int32, tags *VolumeTags) (*VolumeInfo, error) {
./internal/infra/volumes.go:127:func CreateVolumeFromSnapshot(ctx context.Context, clients *AzureClients, resourceGroupName, volumeName, snapshotID string, tags *VolumeTags) (*VolumeInfo, error) {
./internal/infra/volumes.go:185:func DeleteVolume(ctx context.Context, clients *AzureClients, resourceGroupName, volumeName string) error {
./internal/infra/volumes.go:214:func VolumeTagsToMap(tags *VolumeTags) map[string]*string {
./internal/infra/volumes.go:227:func UpdateVolumeStatus(ctx context.Context, clients *AzureClients, volumeID, status string) error {
./internal/infra/volumes.go:270:func UpdateVolumeStatusUserAndBox(ctx context.Context, clients *AzureClients, volumeID, status, userID, boxName string) error {
./internal/infra/resource_graph_queries.go:43:func NewResourceGraphQueries(client *armresourcegraph.Client, subscriptionID, resourceGroup string) *ResourceGraphQueries {
./internal/infra/resource_graph_queries.go:118:func (rq *ResourceGraphQueries) CountInstancesByStatus(ctx context.Context) (*ResourceCounts, error) {
./internal/infra/resource_graph_queries.go:130:func (rq *ResourceGraphQueries) CountVolumesByStatus(ctx context.Context) (*ResourceCounts, error) {
./internal/infra/resource_graph_queries.go:142:func (rq *ResourceGraphQueries) GetInstancesByStatus(ctx context.Context, status string) ([]ResourceInfo, error) {
./internal/infra/resource_graph_queries.go:155:func (rq *ResourceGraphQueries) GetVolumesByStatus(ctx context.Context, status string) ([]ResourceInfo, error) {
./internal/infra/resource_graph_queries.go:168:func (rq *ResourceGraphQueries) GetVolumesByUserAndBox(ctx context.Context, userID, boxName string) ([]ResourceInfo, error) {
./internal/infra/resource_graph_queries.go:185:func (rq *ResourceGraphQueries) GetOldestFreeVolumes(ctx context.Context, limit int) ([]ResourceInfo, error) {
./internal/infra/resource_graph_queries.go:203:func (rq *ResourceGraphQueries) GetRunningInstancesByStatus(ctx context.Context, status string) ([]ResourceInfo, error) {
./internal/infra/resource_graph_queries.go:215:func (rq *ResourceGraphQueries) GetOldestFreeRunningInstances(ctx context.Context, limit int) ([]ResourceInfo, error) {
./internal/infra/resource_graph_queries.go:228:func (rq *ResourceGraphQueries) executeCountQuery(ctx context.Context, query string) (*ResourceCounts, error) {
./internal/infra/resource_graph_queries.go:290:func (rq *ResourceGraphQueries) executeResourceQuery(ctx context.Context, query string) ([]ResourceInfo, error) {
./internal/infra/resource_graph_queries.go:324:func ParseResourceInfo(resourceMap map[string]interface{}) *ResourceInfo {
./internal/infra/resource_graph_queries.go:335:func ParseBasicFields(resource *ResourceInfo, resourceMap map[string]interface{}) {
./internal/infra/resource_graph_queries.go:348:func ParseTags(resource *ResourceInfo, resourceMap map[string]interface{}) {
./internal/infra/resource_graph_queries.go:372:func extractTagValues(resource *ResourceInfo) {
./internal/infra/resource_graph_queries.go:378:func parseTimestamps(resource *ResourceInfo) {
./internal/infra/resource_graph_queries.go:392:func extractResourceID(resource *ResourceInfo) {
./internal/infra/resource_graph_queries.go:402:func ParseProjectedFields(resource *ResourceInfo, resourceMap map[string]interface{}) {
./internal/infra/resource_allocator.go:25:func NewResourceAllocator(clients *AzureClients, resourceQueries *ResourceGraphQueries) *ResourceAllocator {
./internal/infra/resource_allocator.go:34:func (ra *ResourceAllocator) AllocateResourcesForUser(ctx context.Context, userID, boxName string) (*AllocatedResources, error) {
./internal/infra/resource_allocator.go:89:func (ra *ResourceAllocator) ReserveVolumeForUser(ctx context.Context, userID, boxName string) (string, error) {
./internal/infra/resource_allocator.go:110:func (ra *ResourceAllocator) getInstanceIPAndStartQEMU(ctx context.Context, instance, volume *ResourceInfo) (string, error) {
./internal/infra/resource_allocator.go:126:func (ra *ResourceAllocator) rollbackInstanceStatus(ctx context.Context, instanceID string) {
./internal/infra/resource_allocator.go:133:func (ra *ResourceAllocator) rollbackAllocation(ctx context.Context, instanceID, volumeID string) {
./internal/infra/resource_allocator.go:141:func (ra *ResourceAllocator) ReleaseResources(ctx context.Context, instanceID, volumeID string) error {
./internal/infra/resource_naming.go:11:func NewResourceNamer(suffix string) *ResourceNamer {
./internal/infra/resource_naming.go:15:func (r *ResourceNamer) ResourceGroup() string {
./internal/infra/resource_naming.go:19:func (r *ResourceNamer) VNetName() string {
./internal/infra/resource_naming.go:23:func (r *ResourceNamer) BastionSubnetName() string {
./internal/infra/resource_naming.go:27:func (r *ResourceNamer) BoxesSubnetName() string {
./internal/infra/resource_naming.go:31:func (r *ResourceNamer) BastionNSGName() string {
./internal/infra/resource_naming.go:35:func (r *ResourceNamer) BoxNSGName(boxID string) string {
./internal/infra/resource_naming.go:39:func (r *ResourceNamer) BastionVMName() string {
./internal/infra/resource_naming.go:43:func (r *ResourceNamer) BoxVMName(boxID string) string {
./internal/infra/resource_naming.go:47:func (r *ResourceNamer) BoxComputerName(boxID string) string {
./internal/infra/resource_naming.go:54:func (r *ResourceNamer) BastionNICName() string {
./internal/infra/resource_naming.go:58:func (r *ResourceNamer) BoxNICName(boxID string) string {
./internal/infra/resource_naming.go:62:func (r *ResourceNamer) BastionPublicIPName() string {
./internal/infra/resource_naming.go:66:func (r *ResourceNamer) BastionOSDiskName() string {
./internal/infra/resource_naming.go:70:func (r *ResourceNamer) BoxOSDiskName(boxID string) string {
./internal/infra/resource_naming.go:74:func (r *ResourceNamer) BoxDataDiskName(boxID string) string {
./internal/infra/resource_naming.go:78:func (r *ResourceNamer) VolumePoolDiskName(volumeID string) string {
./internal/infra/resource_naming.go:83:func (r *ResourceNamer) EventLogTableName() string {
./internal/infra/resource_naming.go:89:func (r *ResourceNamer) ResourceRegistryTableName() string {
./internal/infra/resource_naming.go:96:func (r *ResourceNamer) cleanSuffixForTable() string {
./internal/infra/resource_naming.go:103:func (r *ResourceNamer) cleanSuffixAlphanumeric(allowUppercase bool) string {
./internal/infra/bastion.go:37:func GenerateBastionInitScript() (string, error) {
./internal/infra/bastion.go:44:func compileBastionServer() error {
./internal/infra/bastion.go:51:func CreateBastionPublicIP(ctx context.Context, clients *AzureClients) (*armnetwork.PublicIPAddress, error) {
./internal/infra/bastion.go:72:func CreateBastionNIC(ctx context.Context, clients *AzureClients, publicIPID *string) (*armnetwork.Interface, error) {
./internal/infra/bastion.go:103:func CreateBastionVM(ctx context.Context, clients *AzureClients, config *VMConfig, nicID, customData string) (*armcompute.VirtualMachine, error) {
./internal/infra/bastion.go:176:func copyServerBinary(ctx context.Context, config *VMConfig, publicIPAddress string) error {
./internal/infra/bastion.go:187:func copyTableStorageConfig(ctx context.Context, clients *AzureClients, config *VMConfig, publicIPAddress string) error {
./internal/infra/bastion.go:207:func copySSHKeyToBastion(ctx context.Context, config *VMConfig, bastionIP string) error {
./internal/infra/bastion.go:262:func startServerOnBastion(ctx context.Context, config *VMConfig, publicIPAddress, resourceGroupSuffix string) error {
./internal/infra/bastion.go:269:func getBastionRoleID(subscriptionID string) string {
./internal/infra/bastion.go:274:func assignRoleToVM(ctx context.Context, clients *AzureClients, principalID *string) error {
./internal/infra/bastion.go:301:func DeployBastion(ctx context.Context, clients *AzureClients, config *VMConfig) string {
./internal/sshserver/commands.go:37:func parseCommand(cmdLine string) CommandResult {
./internal/sshserver/commands.go:79:func createCobraCommand(result *CommandResult) *cobra.Command {
./internal/sshserver/commands.go:165:func ParseArgs(cmdLine string) []string {
./internal/sshserver/server.go:31:func New(port int, clients *infra.AzureClients) (*Server, error) {
./internal/sshserver/server.go:72:func (s *Server) dialBoxAtIP(boxIP string) (*ssh.Client, error) {
./internal/sshserver/server.go:94:func (s *Server) handleSCP(_ gssh.Session) error {
./internal/sshserver/server.go:101:func (s *Server) handleSession(sess gssh.Session) {
./internal/sshserver/server.go:145:func (s *Server) handleShellSession(ctx CommandContext, sess gssh.Session, resources *infra.AllocatedResources) {
./internal/sshserver/server.go:225:func (s *Server) setupPty(sess gssh.Session, boxSession *ssh.Session) error {
./internal/sshserver/server.go:242:func (s *Server) handleIO(sess gssh.Session, boxSession *ssh.Session) error {
./internal/sshserver/server.go:286:func (s *Server) handleCommandSession(sess gssh.Session) {
./internal/sshserver/server.go:331:func generateUserID(publicKey ssh.PublicKey) string {
./internal/sshserver/server.go:341:func (s *Server) createCommandContext(sess gssh.Session) CommandContext {
./internal/sshserver/server.go:352:func (s *Server) handleSpinupCommand(ctx CommandContext, result CommandResult, sess gssh.Session) {
./internal/sshserver/server.go:395:func (s *Server) handleConnectCommand(ctx CommandContext, result CommandResult, sess gssh.Session) {
./internal/sshserver/server.go:429:func (s *Server) handleHelpCommand(_ CommandContext, _ CommandResult, sess gssh.Session) {
./internal/sshserver/server.go:457:func (s *Server) handleVersionCommand(_ CommandContext, _ CommandResult, sess gssh.Session) {
./internal/sshserver/server.go:469:func (s *Server) handleWhoamiCommand(ctx CommandContext, _ CommandResult, sess gssh.Session) {
./internal/sshserver/server.go:493:func (s *Server) Run() error {
./internal/sshutil/ssh.go:22:func LoadKeyPair() (privateKey, publicKey string, err error) {
./internal/sshutil/ssh.go:80:func CopyFile(ctx context.Context, localPath, remotePath, username, hostname string) error {
./internal/sshutil/ssh.go:90:func ExecuteCommand(ctx context.Context, command, username, hostname string) error {
./internal/sshutil/ssh.go:103:func ExecuteCommandWithOutput(ctx context.Context, command, username, hostname string) (string, error) {
