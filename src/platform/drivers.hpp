#pragma once
#include "platform/service.hpp"
namespace fpga {
TableResult driverStatus(Context& context);
void validateWriterPackage(const std::filesystem::path& path);
bool changeDriver(const DriverRequest& request, Context& context);
Notice manageWindows(const Request& request, Context& context);
int runAdministratorFile(const std::filesystem::path& path, const std::string& expectedHash);
int runAdministrator(const std::wstring& encodedRequest);
}
