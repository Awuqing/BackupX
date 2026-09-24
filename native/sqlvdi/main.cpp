// SQL Server VDI media worker. SQL commands and credentials stay in the Go process.
#include <atomic>
#include <chrono>
#include <cstdio>
#include <cstring>
#include <stdexcept>
#include <string>
#include <thread>

#ifdef _WIN32
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#include <objbase.h>
#include <io.h>
#include <fcntl.h>
#include <sys/stat.h>
#include "vdi.h"
#include "vdierror.h"
#include "vdiguid.h"
using Device = IClientVirtualDevice;
#else
#include <fcntl.h>
#include <unistd.h>
#include "vdi.h"
#include "vdierror.h"
using Device = ClientVirtualDevice;
#endif

namespace {
std::atomic<bool> cancelled{false};
using Clock = std::chrono::steady_clock;
// These protocol values also work with the older Windows SDK header.
constexpr unsigned requestComplete = 0x2000;
constexpr unsigned completeCommand = 19;

void check(int status, const char* stage) {
    if (status != 0) {
        char message[128];
        std::snprintf(message, sizeof(message), "%s failed (VDI 0x%08x)", stage, static_cast<unsigned>(status));
        throw std::runtime_error(message);
    }
}

#ifdef _WIN32
std::wstring wide(const std::string& text) {
    if (text.empty()) return {};
    int size = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text.data(), static_cast<int>(text.size()), nullptr, 0);
    if (!size) throw std::runtime_error("invalid UTF-8 argument");
    std::wstring value(size, L'\0');
    if (!MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text.data(), static_cast<int>(text.size()), value.data(), size)) {
        throw std::runtime_error("UTF-8 conversion failed");
    }
    return value;
}
#endif

struct DeviceSet {
#ifdef _WIN32
    IClientVirtualDeviceSet2* set = nullptr;
    DeviceSet() {
        auto initialized = CoInitializeEx(nullptr, COINIT_MULTITHREADED);
        if (FAILED(initialized)) check(initialized, "CoInitializeEx");
        auto status = CoCreateInstance(CLSID_MSSQL_ClientVirtualDeviceSet, nullptr, CLSCTX_INPROC_SERVER,
                                       IID_IClientVirtualDeviceSet2, reinterpret_cast<void**>(&set));
        if (status != 0) {
            CoUninitialize();
            check(status, "CoCreateInstance (install/register SQL Server VDI x64)");
        }
    }
    ~DeviceSet() { set->Release(); CoUninitialize(); }
    void create(const std::string& name, const std::string& instance, VDConfig& config) {
        auto n = wide(name), i = wide(instance);
        check(set->CreateEx(i.empty() ? nullptr : i.c_str(), n.c_str(), &config), "CreateEx");
    }
    Device* open(const std::string& name) {
        Device* device = nullptr;
        check(set->OpenDevice(wide(name).c_str(), &device), "OpenDevice");
        return device;
    }
#else
    ClientVirtualDeviceSet storage;
    ClientVirtualDeviceSet* set = &storage;
    void create(const std::string& name, const std::string& instance, VDConfig& config) {
        if (!instance.empty()) throw std::runtime_error("named instances are Windows-only");
        check(set->Create(const_cast<char*>(name.c_str()), &config), "Create (run as mssql or configure shared-memory permissions)");
    }
    Device* open(const std::string& name) {
        Device* device = nullptr;
        check(set->OpenDevice(const_cast<char*>(name.c_str()), &device), "OpenDevice");
        return device;
    }
#endif
};

struct Media {
    FILE* file = nullptr;
    bool backup;
    Media(const std::string& path, bool write) : backup(write) {
#ifdef _WIN32
        int fd = _wopen(wide(path).c_str(), _O_BINARY | (write ? _O_WRONLY | _O_CREAT | _O_EXCL : _O_RDONLY), _S_IREAD | _S_IWRITE);
        if (fd >= 0) {
            file = _fdopen(fd, write ? "wb" : "rb");
            if (!file && _close(fd) != 0) std::fprintf(stderr, "close media descriptor failed\n");
        }
#else
        int fd = ::open(path.c_str(), write ? O_WRONLY | O_CREAT | O_EXCL : O_RDONLY, 0600);
        if (fd >= 0) {
            file = fdopen(fd, write ? "wb" : "rb");
            if (!file && ::close(fd) != 0) std::fprintf(stderr, "close media descriptor failed\n");
        }
#endif
        if (!file) throw std::runtime_error("open media failed (backup output must not already exist)");
    }
    ~Media() { if (file && fclose(file) != 0) std::fprintf(stderr, "close media failed\n"); }
    bool flush() {
        if (!file || !backup) return true;
        if (fflush(file) != 0) return false;
#ifdef _WIN32
        return _commit(_fileno(file)) == 0;
#else
        return fsync(fileno(file)) == 0;
#endif
    }
    bool finish() {
        if (!file) return true;
        bool ok = flush();
        if (fclose(file) != 0) ok = false;
        file = nullptr;
        return ok;
    }
};

void transfer(Device* device, Media& media) {
    auto lastCommand = Clock::now();
    while (!cancelled.load()) {
        VDC_Command* command = nullptr;
        int status = device->GetCommand(1000, &command);
        if (status == VD_E_CLOSE) {
            if (!media.finish()) throw std::runtime_error("final media flush/close failed");
            return;
        }
        if (status == VD_E_TIMEOUT) {
            if (Clock::now() - lastCommand > std::chrono::minutes(5)) throw std::runtime_error("VDI transfer idle timeout");
            continue;
        }
        check(status, "GetCommand");
        lastCommand = Clock::now();
        if (!command || ((command->commandCode == VDC_Read || command->commandCode == VDC_Write) &&
                         (static_cast<int64_t>(command->size) < 0 || (command->size && !command->buffer)))) {
            throw std::runtime_error("invalid VDI command buffer");
        }
        unsigned code = ERROR_SUCCESS;
        size_t bytes = 0;
        switch (command->commandCode) {
        case VDC_Write:
            if (!media.backup || !media.file) { code = ERROR_NOT_SUPPORTED; break; }
            bytes = fwrite(command->buffer, 1, command->size, media.file);
            if (bytes != static_cast<size_t>(command->size)) code = ERROR_DISK_FULL;
            break;
        case VDC_Read:
            if (media.backup || !media.file) { code = ERROR_NOT_SUPPORTED; break; }
            bytes = fread(command->buffer, 1, command->size, media.file);
            if (ferror(media.file)) code = 30; // Win32 ERROR_READ_FAULT
            else if (bytes != static_cast<size_t>(command->size)) code = ERROR_HANDLE_EOF;
            break;
        case VDC_Flush:
            if (!media.flush()) code = ERROR_DISK_FULL;
            break;
        case VDC_ClearError:
            // Never discard a real media failure: failures terminate below.
            break;
        case completeCommand:
            if (!media.finish()) code = ERROR_DISK_FULL;
            break;
        default:
            code = ERROR_NOT_SUPPORTED;
        }
        check(device->CompleteCommand(command, code, static_cast<unsigned long>(bytes), 0), "CompleteCommand");
        if (code != ERROR_SUCCESS && code != ERROR_HANDLE_EOF) throw std::runtime_error("VDI media command failed");
    }
    throw std::runtime_error("VDI operation cancelled");
}

int run(int argc, char** argv) {
    if (argc != 5 || (std::strcmp(argv[1], "backup") && std::strcmp(argv[1], "restore"))) {
        throw std::runtime_error("usage: backupx-sqlvdi backup|restore device-name artifact-path instance-name");
    }
    Media media(argv[3], std::strcmp(argv[1], "backup") == 0);
    DeviceSet devices;
    VDConfig config{};
    config.deviceCount = 1;
    config.features = requestComplete;
    devices.create(argv[2], argv[4], config);
    try {
        // EOF is the cross-platform cancellation signal, including parent-process death.
        std::thread([] { while (std::getchar() != EOF) {} cancelled.store(true); }).detach();
        if (std::fputs("BACKUPX_SQLVDI_READY\n", stdout) < 0 || std::fflush(stdout) != 0) throw std::runtime_error("write readiness failed");
        auto deadline = Clock::now() + std::chrono::seconds(60);
        for (;;) {
            if (cancelled.load()) throw std::runtime_error("VDI configuration cancelled");
            int status = devices.set->GetConfiguration(1000, &config);
            if (status == VD_E_TIMEOUT && Clock::now() < deadline) continue;
            check(status, "GetConfiguration");
            break;
        }
        transfer(devices.open(argv[2]), media);
    } catch (...) {
        int status = devices.set->SignalAbort();
        if (status != 0) std::fprintf(stderr, "SignalAbort failed: 0x%08x\n", static_cast<unsigned>(status));
        status = devices.set->Close();
        if (status != 0) std::fprintf(stderr, "Close after abort failed: 0x%08x\n", static_cast<unsigned>(status));
        throw;
    }
    check(devices.set->Close(), "Close");
    return 0;
}
} // namespace

#ifdef _WIN32
// CRT narrow argv uses the system code page; convert UTF-16 arguments explicitly.
int wmain(int argc, wchar_t** argv) {
    try {
        std::string args[5];
        char* pointers[5];
        if (argc != 5) throw std::runtime_error("expected four arguments");
        for (int i = 0; i < argc; ++i) {
            int size = WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, argv[i], -1, nullptr, 0, nullptr, nullptr);
            if (!size) throw std::runtime_error("invalid UTF-16 argument");
            args[i].resize(size);
            if (!WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, argv[i], -1, args[i].data(), size, nullptr, nullptr)) throw std::runtime_error("argument conversion failed");
            pointers[i] = args[i].data();
        }
        return run(argc, pointers);
    } catch (const std::exception& e) { std::fprintf(stderr, "%s\n", e.what()); return 1; }
}
#else
int main(int argc, char** argv) {
    try { return run(argc, argv); }
    catch (const std::exception& e) { std::fprintf(stderr, "%s\n", e.what()); return 1; }
}
#endif
