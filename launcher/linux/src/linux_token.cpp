#include "linux_token.h"

#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QUuid>

#include <fcntl.h>
#include <sys/stat.h>
#include <unistd.h>

namespace sapie::lnx {
namespace {

QString fail(QString* error, const QString& message)
{
    if (error) {
        *error = message;
    }
    return {};
}

}

bool writeOwnerOnlyFile(const QString& path, const QByteArray& contents, QString* error)
{
    const QFileInfo destination(path);
    if (destination.isDir() || destination.isSymLink()) {
        return fail(error, QStringLiteral("The token path is a directory or a link.")), false;
    }
    if (!QDir().mkpath(destination.absolutePath())) {
        return fail(error, QStringLiteral("The token directory could not be created.")), false;
    }

    const QString temporary = destination.absolutePath() + QStringLiteral("/.sapie-token-")
        + QUuid::createUuid().toString(QUuid::WithoutBraces);
    const int fd = ::open(temporary.toUtf8().constData(),
                          O_WRONLY | O_CREAT | O_EXCL | O_CLOEXEC, 0600);
    if (fd < 0) {
        return fail(error, QStringLiteral("A private token file could not be created.")), false;
    }

    size_t offset = 0;
    bool written = true;
    while (offset < static_cast<size_t>(contents.size())) {
        const ssize_t count = ::write(fd, contents.constData() + offset,
                                      static_cast<size_t>(contents.size()) - offset);
        if (count <= 0) {
            written = false;
            break;
        }
        offset += static_cast<size_t>(count);
    }
    if (written && (::fchmod(fd, 0600) != 0 || ::fsync(fd) != 0)) {
        written = false;
    }
    if (::close(fd) != 0) {
        written = false;
    }
    if (!written) {
        ::unlink(temporary.toUtf8().constData());
        return fail(error, QStringLiteral("The private token file could not be written.")), false;
    }

    QFile::setPermissions(temporary, QFileDevice::ReadOwner | QFileDevice::WriteOwner);
    if (!QFile::rename(temporary, destination.absoluteFilePath())) {
        ::unlink(temporary.toUtf8().constData());
        return fail(error, QStringLiteral("The token file could not be replaced.")), false;
    }
    return true;
}

bool verifyOwnerOnlyFile(const QString& path, QString* error)
{
    struct stat info {};
    if (::lstat(path.toUtf8().constData(), &info) != 0 || !S_ISREG(info.st_mode)) {
        return fail(error, QStringLiteral("The token file is missing or not a regular file.")), false;
    }
    if (info.st_mode & (S_IRWXG | S_IRWXO)) {
        return fail(error, QStringLiteral("The token file is readable or writable by another user.")), false;
    }
    if ((info.st_mode & (S_IRUSR | S_IWUSR)) != (S_IRUSR | S_IWUSR)) {
        return fail(error, QStringLiteral("The token file has an unexpected owner mode.")), false;
    }
    return true;
}

}
