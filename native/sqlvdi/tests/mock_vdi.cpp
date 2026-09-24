// Link-only VDI stand-in: exercises the production worker without SQL Server.
#include "vdi.h"
#include "vdierror.h"
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <thread>

namespace {
const char* scenario() { return std::getenv("BACKUPX_TEST_VDI_CASE"); }
void log(const char* text) {
    FILE* file = std::fopen(std::getenv("BACKUPX_TEST_VDI_LOG"), "a");
    if (!file) std::abort();
    if (std::fprintf(file, "%s\n", text) < 0 || std::fclose(file) != 0) std::abort();
}
}
class CVD {
public:
    int step = 0;
    uint8_t buffer[6] = {'b','a','c','k','u','p'};
    VDC_Command cmd{};
};
class CVDS { public: ClientVirtualDevice device; };
ClientVirtualDevice::ClientVirtualDevice() : cvd(new CVD) {}
ClientVirtualDevice::~ClientVirtualDevice() { delete cvd; }
ClientVirtualDeviceSet::ClientVirtualDeviceSet() : cvds(new CVDS) {}
ClientVirtualDeviceSet::~ClientVirtualDeviceSet() { delete cvds; }
int ClientVirtualDeviceSet::Create(char*, VDConfig* config) {
    if (config->deviceCount != 1 || config->features != VDF_RequestComplete) return VD_E_INVALID;
    return std::strcmp(scenario(), "create_fail") == 0 ? VD_E_SECURITY : 0;
}
int ClientVirtualDeviceSet::GetConfiguration(time_t, VDConfig*) {
    if (std::strcmp(scenario(), "cancel") == 0) {
        std::this_thread::sleep_for(std::chrono::milliseconds(5));
        return VD_E_TIMEOUT;
    }
    return 0;
}
int ClientVirtualDeviceSet::OpenDevice(char*, ClientVirtualDevice** device) { *device = &cvds->device; return 0; }
int ClientVirtualDeviceSet::Close() { log("close"); return 0; }
int ClientVirtualDeviceSet::SignalAbort() { log("abort"); return 0; }
int ClientVirtualDevice::GetCommand(time_t, VDC_Command** command) {
    int step = cvd->step++;
    cvd->cmd.size = sizeof(cvd->buffer);
    cvd->cmd.buffer = cvd->buffer;
    if (step == 0) {
        cvd->cmd.commandCode = std::strcmp(scenario(), "restore") == 0 ? VDC_Read : VDC_Write;
        if (std::strcmp(scenario(), "unknown_command") == 0) cvd->cmd.commandCode = 999;
    } else if (step == 1) cvd->cmd.commandCode = VDC_Flush;
    else if (step == 2 && std::strcmp(scenario(), "legacy") != 0) cvd->cmd.commandCode = VDC_Complete;
    else return std::strcmp(scenario(), "abort") == 0 ? VD_E_ABORT : VD_E_CLOSE;
    *command = &cvd->cmd;
    return 0;
}
int ClientVirtualDevice::CompleteCommand(VDC_Command* command, int code, unsigned long transferred, int64_t) {
    if (std::strcmp(scenario(), "unknown_command") == 0) return code == ERROR_NOT_SUPPORTED ? 0 : VD_E_INVALID;
    if (code != 0) return VD_E_ABORT;
    if (command->commandCode == VDC_Read || command->commandCode == VDC_Write) {
        if (transferred != sizeof(cvd->buffer) || std::memcmp(cvd->buffer, "backup", 6) != 0) return VD_E_INVALID;
    }
    if (command->commandCode == VDC_Complete) log("complete");
    return 0;
}
