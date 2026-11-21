#include "win_acl.h"

#include "win_native.h"

#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QUuid>

#include <windows.h>
#include <aclapi.h>
#include <sddl.h>

#include <vector>

namespace sapie::win {
namespace {

QString systemError(const QString& action)
{
    return QStringLiteral("%1 failed with Windows error %2").arg(action).arg(GetLastError());
}

bool currentSid(QByteArray* sid)
{
    HANDLE token = nullptr;
    if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token)) {
        return false;
    }

    DWORD length = 0;
    GetTokenInformation(token, TokenUser, nullptr, 0, &length);
    std::vector<unsigned char> buffer(length ? length : 1);
    TOKEN_USER* user = reinterpret_cast<TOKEN_USER*>(buffer.data());
    const bool ok = length != 0
        && GetTokenInformation(token, TokenUser, user, length, &length)
        && IsValidSid(user->User.Sid);
    CloseHandle(token);
    if (!ok) {
        return false;
    }

    const DWORD sidLength = GetLengthSid(user->User.Sid);
    sid->resize(static_cast<int>(sidLength));
    if (!CopySid(sidLength, sid->data(), user->User.Sid)) {
        sid->clear();
        return false;
    }
    return true;
}

bool buildOwnerAcl(QByteArray* owner, PACL* acl)
{
    QByteArray sid;
    if (!currentSid(&sid)) {
        return false;
    }
    return ownerAcl(sid, owner, acl);
}

bool ownerAcl(const QByteArray& sid, QByteArray* owner, PACL* acl)
{
    *owner = sid;
    EXPLICIT_ACCESS_W entry{};
    entry.grfAccessPermissions = FILE_ALL_ACCESS;
    entry.grfAccessMode = GRANT_ACCESS;
    entry.grfInheritance = NO_INHERITANCE;
    entry.Trustee.TrusteeForm = TRUSTEE_IS_SID;
    entry.Trustee.TrusteeType = TRUSTEE_IS_USER;
    entry.Trustee.ptstrName = reinterpret_cast<LPWSTR>(owner->data());

    const DWORD result = SetEntriesInAclW(1, &entry, nullptr, acl);
    if (result != ERROR_SUCCESS) {
        SetLastError(result);
        return false;
    }
    return *acl != nullptr;
}

bool writeAll(HANDLE handle, const QByteArray& contents)
{
    DWORD offset = 0;
    while (offset < static_cast<DWORD>(contents.size())) {
        DWORD written = 0;
        if (!WriteFile(handle, contents.constData() + offset,
                       static_cast<DWORD>(contents.size()) - offset, &written, nullptr)
            || written == 0) {
            return false;
        }
        offset += written;
    }
    return FlushFileBuffers(handle) != FALSE;
}

}

bool writeOwnerOnlyFile(const QString& path, const QByteArray& contents, QString* error)
{
    const QString destination = QFileInfo(path).absoluteFilePath();
    QDir().mkpath(QFileInfo(destination).absolutePath());
    const QString temporary = destination + QStringLiteral(".sapie-tmp-")
        + QUuid::createUuid().toString(QUuid::WithoutBraces);

    QByteArray owner;
    PACL acl = nullptr;
    if (!buildOwnerAcl(&owner, &acl)) {
        if (error) {
            *error = systemError(QStringLiteral("Building the owner-only ACL"));
        }
        return false;
    }

    SECURITY_DESCRIPTOR descriptor{};
    if (!InitializeSecurityDescriptor(&descriptor, SECURITY_DESCRIPTOR_REVISION)
        || !SetSecurityDescriptorDacl(&descriptor, TRUE, acl, FALSE)
        || !SetSecurityDescriptorOwner(&descriptor, owner.data(), FALSE)
        || !SetSecurityDescriptorControl(&descriptor, SE_DACL_PROTECTED, SE_DACL_PROTECTED)) {
        LocalFree(acl);
        if (error) {
            *error = systemError(QStringLiteral("Preparing the token file descriptor"));
        }
        return false;
    }

    SECURITY_ATTRIBUTES attributes{};
    attributes.nLength = sizeof(attributes);
    attributes.lpSecurityDescriptor = &descriptor;
    attributes.bInheritHandle = FALSE;

    const std::wstring nativeTemporary = extendedLengthPath(temporary).toStdWString();
    HANDLE handle = CreateFileW(nativeTemporary.c_str(), GENERIC_WRITE, FILE_SHARE_READ,
                                &attributes, CREATE_NEW,
                                FILE_ATTRIBUTE_TEMPORARY | FILE_FLAG_WRITE_THROUGH, nullptr);
    if (handle == INVALID_HANDLE_VALUE) {
        LocalFree(acl);
        if (error) {
            *error = systemError(QStringLiteral("Creating the token file"));
        }
        return false;
    }
    LocalFree(acl);

    const bool written = writeAll(handle, contents);
    CloseHandle(handle);
    if (!written) {
        QFile::remove(temporary);
        if (error) {
            *error = systemError(QStringLiteral("Writing the token file"));
        }
        return false;
    }

    const std::wstring nativeDestination = extendedLengthPath(destination).toStdWString();
    const bool moved = MoveFileExW(nativeTemporary.c_str(), nativeDestination.c_str(),
                                   MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH);
    if (!moved) {
        QFile::remove(temporary);
        if (error) {
            *error = systemError(QStringLiteral("Replacing the token file"));
        }
        return false;
    }
    if (!verifyOwnerOnlyFile(destination, error)) {
        QFile::remove(destination);
        return false;
    }
    return true;
}

bool verifyOwnerOnlyFile(const QString& path, QString* error)
{
    const QString failure = QStringLiteral("The token file ACL is not owner-only.");
    QByteArray expectedSid;
    if (!currentSid(&expectedSid)) {
        if (error) {
            *error = systemError(QStringLiteral("Reading the current user SID"));
        }
        return false;
    }

    const std::wstring native = extendedLengthPath(path).toStdWString();
    HANDLE handle = CreateFileW(native.c_str(), READ_CONTROL, FILE_SHARE_READ | FILE_SHARE_WRITE,
                                nullptr, OPEN_EXISTING, FILE_FLAG_BACKUP_SEMANTICS, nullptr);
    if (handle == INVALID_HANDLE_VALUE) {
        if (error) {
            *error = systemError(QStringLiteral("Opening the token file ACL"));
        }
        return false;
    }

    PSECURITY_DESCRIPTOR descriptor = nullptr;
    PSID owner = nullptr;
    PACL acl = nullptr;
    const DWORD result = GetSecurityInfo(handle, SE_FILE_OBJECT,
                                         OWNER_SECURITY_INFORMATION | DACL_SECURITY_INFORMATION,
                                         &owner, nullptr, &acl, nullptr, &descriptor);
    CloseHandle(handle);
    if (result != ERROR_SUCCESS || !descriptor || !owner || !acl) {
        LocalFree(descriptor);
        if (error) {
            *error = failure;
        }
        return false;
    }

    SECURITY_DESCRIPTOR_CONTROL control = 0;
    DWORD revision = 0;
    ACL_SIZE_INFORMATION size{};
    bool valid = GetSecurityDescriptorControl(descriptor, &control, &revision)
        && (control & SE_DACL_PROTECTED)
        && IsValidSid(owner)
        && EqualSid(owner, expectedSid.data())
        && GetAclInformation(acl, &size, sizeof(size), AclSizeInformation)
        && size.AceCount == 1;

    if (valid) {
        LPVOID rawAce = nullptr;
        valid = GetAce(acl, 0, &rawAce) && rawAce != nullptr;
        if (valid) {
            const ACCESS_ALLOWED_ACE* ace = reinterpret_cast<const ACCESS_ALLOWED_ACE*>(rawAce);
            const DWORD required = FILE_GENERIC_READ | FILE_GENERIC_WRITE | DELETE;
            valid = ace->Header.AceType == ACCESS_ALLOWED_ACE_TYPE
                && ace->Header.AceFlags == 0
                && IsValidSid(&ace->SidStart)
                && EqualSid(&ace->SidStart, expectedSid.data())
                && (ace->Mask & required) == required;
        }
    }

    LocalFree(descriptor);
    if (!valid && error) {
        *error = failure;
    }
    return valid;
}

}
