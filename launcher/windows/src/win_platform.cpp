#include "win_platform.h"

#include "win_native.h"

#include <QByteArray>
#include <QLocalServer>
#include <QLocalSocket>
#include <QMenu>
#include <QSystemTrayIcon>
#include <QWindow>

#include <windows.h>
#include <objbase.h>
#include <propkey.h>
#include <propvarutil.h>
#include <sddl.h>
#include <shobjidl.h>
#include <shobjidl_core.h>

#include <memory>
#include <vector>

namespace sapie::win {
namespace {

template <typename T>
class ComPtr {
public:
    ComPtr() = default;
    ~ComPtr() { reset(); }
    ComPtr(const ComPtr&) = delete;
    ComPtr& operator=(const ComPtr&) = delete;

    T** out() { return &m_value; }
    T* get() const { return m_value; }
    explicit operator bool() const { return m_value != nullptr; }

    void releaseOwnership() { m_value = nullptr; }

private:
    void reset()
    {
        if (m_value) {
            m_value->Release();
            m_value = nullptr;
        }
    }

    T* m_value = nullptr;
};

}

WindowsPlatform::WindowsPlatform(QObject* parent)
    : QObject(parent)
{
    m_comInitialised = SUCCEEDED(CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED));
    setAppUserModelId();
}

WindowsPlatform::~WindowsPlatform()
{
    if (m_taskbar) {
        static_cast<IUnknown*>(m_taskbar)->Release();
        m_taskbar = nullptr;
    }
    if (m_comInitialised) {
        CoUninitialize();
    }
}

bool WindowsPlatform::reducedMotion() const
{
    BOOL animations = TRUE;
    if (!SystemParametersInfoW(SPI_GETCLIENTAREAANIMATION, 0, &animations, 0)) {
        return false;
    }
    return animations == FALSE;
}

QString WindowsPlatform::dataDirectory() const
{
    return win::dataDirectory();
}

bool WindowsPlatform::registerIntegrations(const QStringList& fileExtensions, QString* error)
{
    if (!registerProtocol(error)) {
        return false;
    }
    return registerFileAssociations(fileExtensions, error);
}

bool WindowsPlatform::unregisterIntegrations(const QStringList& fileExtensions, QString* error)
{
    const bool protocolRemoved = unregisterProtocol(error);
    const bool filesRemoved = unregisterFileAssociations(fileExtensions, error);
    return protocolRemoved && filesRemoved;
}

void WindowsPlatform::attachWindow(QWindow* window)
{
    if (!window) {
        return;
    }
    applyDarkTitleBar(window);
    m_window = reinterpret_cast<void*>(window->winId());
    if (m_taskbar || !m_comInitialised) {
        return;
    }

    ComPtr<ITaskbarList3> taskbar;
    if (SUCCEEDED(CoCreateInstance(CLSID_TaskbarList, nullptr, CLSCTX_INPROC_SERVER,
                                   IID_PPV_ARGS(taskbar.out())))
        && SUCCEEDED(taskbar.get()->HrInit())) {
        m_taskbar = taskbar.get();
        taskbar.releaseOwnership();
    }
}

void WindowsPlatform::installTray(const QIcon& icon, QMenu* menu)
{
    if (!m_tray) {
        m_tray = new QSystemTrayIcon(icon, this);
        connect(m_tray, &QSystemTrayIcon::activated, this, [this](QSystemTrayIcon::ActivationReason reason) {
            emit trayActivated(reason == QSystemTrayIcon::Trigger
                               || reason == QSystemTrayIcon::DoubleClick);
        });
    } else {
        m_tray->setIcon(icon);
    }
    m_tray->setContextMenu(menu);
    m_tray->setToolTip(QStringLiteral("SAPIE"));
    m_tray->show();
}

void WindowsPlatform::setJumpList(const QVector<JumpTask>& tasks, QString* error)
{
    if (!m_comInitialised) {
        if (error) {
            *error = QStringLiteral("COM is not available for the Windows jump list.");
        }
        return;
    }

    ComPtr<ICustomDestinationList> destination;
    HRESULT result = CoCreateInstance(CLSID_DestinationList, nullptr, CLSCTX_INPROC_SERVER,
                                      IID_PPV_ARGS(destination.out()));
    if (FAILED(result)) {
        if (error) {
            *error = QStringLiteral("The Windows jump list could not be opened.");
        }
        return;
    }

    UINT maximumSlots = 0;
    ComPtr<IObjectArray> removed;
    result = destination.get()->BeginList(&maximumSlots, IID_PPV_ARGS(removed.out()));
    if (FAILED(result)) {
        if (error) {
            *error = QStringLiteral("The Windows jump list could not be started.");
        }
        return;
    }

    ComPtr<IObjectCollection> collection;
    result = CoCreateInstance(CLSID_EnumerableObjectCollection, nullptr, CLSCTX_INPROC_SERVER,
                              IID_PPV_ARGS(collection.out()));
    if (FAILED(result)) {
        destination.get()->AbortList();
        if (error) {
            *error = QStringLiteral("The Windows jump list tasks could not be built.");
        }
        return;
    }

    for (const JumpTask& task : tasks) {
        ComPtr<IShellLinkW> link;
        if (FAILED(CoCreateInstance(CLSID_ShellLink, nullptr, CLSCTX_INPROC_SERVER,
                                    IID_PPV_ARGS(link.out())))) {
            continue;
        }
        const std::wstring path = extendedLengthPath(task.executable).toStdWString();
        link.get()->SetPath(path.c_str());
        link.get()->SetArguments(task.arguments.toStdWString().c_str());
        link.get()->SetDescription(task.title.toStdWString().c_str());
        if (!task.icon.isEmpty()) {
            link.get()->SetIconLocation(task.icon.toStdWString().c_str(), 0);
        }

        ComPtr<IPropertyStore> properties;
        if (SUCCEEDED(link.get()->QueryInterface(IID_PPV_ARGS(properties.out())))) {
            PROPVARIANT title{};
            if (SUCCEEDED(InitPropVariantFromString(task.title.toStdWString().c_str(), &title))) {
                properties.get()->SetValue(PKEY_Title, title);
                PropVariantClear(&title);
            }
        }
        collection.get()->AddObject(link.get());
    }

    ComPtr<IObjectArray> taskArray;
    if (FAILED(collection.get()->QueryInterface(IID_PPV_ARGS(taskArray.out())))
        || FAILED(destination.get()->AddUserTasks(taskArray.get()))
        || FAILED(destination.get()->CommitList(&maximumSlots))) {
        destination.get()->AbortList();
        if (error) {
            *error = QStringLiteral("The Windows jump list could not be committed.");
        }
        return;
    }
}

void WindowsPlatform::setProgress(int value, int maximum)
{
    if (!m_taskbar || !m_window) {
        return;
    }
    auto* taskbar = static_cast<ITaskbarList3*>(m_taskbar);
    const HWND handle = static_cast<HWND>(m_window);
    if (maximum <= 0 || value <= 0) {
        taskbar->SetProgressState(handle, TBPF_NOPROGRESS);
        return;
    }
    const int bounded = qBound(0, value, maximum);
    taskbar->SetProgressState(handle, TBPF_NORMAL);
    taskbar->SetProgressValue(handle, static_cast<ULONGLONG>(bounded), static_cast<ULONGLONG>(maximum));
}

void WindowsPlatform::clearProgress()
{
    setProgress(0, 0);
}

QString WindowsPlatform::currentSid() const
{
    HANDLE token = nullptr;
    if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token)) {
        return {};
    }

    DWORD length = 0;
    GetTokenInformation(token, TokenUser, nullptr, 0, &length);
    std::vector<unsigned char> buffer(length ? length : 1);
    auto* user = reinterpret_cast<TOKEN_USER*>(buffer.data());
    if (length == 0 || !GetTokenInformation(token, TokenUser, user, length, &length)) {
        CloseHandle(token);
        return {};
    }
    CloseHandle(token);

    LPWSTR text = nullptr;
    if (!ConvertSidToStringSidW(user->User.Sid, &text)) {
        return {};
    }
    const QString sid = QString::fromWCharArray(text);
    LocalFree(text);
    return sid;
}

bool WindowsPlatform::listenForSecondInstance(QString* error)
{
    if (m_localServer) {
        return true;
    }
    const QString sid = currentSid();
    if (sid.isEmpty()) {
        if (error) {
            *error = QStringLiteral("Could not read the current user SID.");
        }
        return false;
    }
    const QString name = QStringLiteral("sapie-%1-local").arg(sid);
    auto* server = new QLocalServer(this);
    if (!server->listen(name)) {
        QLocalServer::removeServer(name);
        if (!server->listen(name)) {
            if (error) {
                *error = QStringLiteral("Could not open the single-instance pipe.");
            }
            delete server;
            return false;
        }
    }
    connect(server, &QLocalServer::newConnection, this, &WindowsPlatform::onNewLocalConnection);
    m_localServer = server;
    return true;
}

bool WindowsPlatform::sendToRunningInstance(const QStringList& arguments, QString* error)
{
    HANDLE token = nullptr;
    if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token)) {
        if (error) {
            *error = QStringLiteral("Could not read the current user SID.");
        }
        return false;
    }
    DWORD length = 0;
    GetTokenInformation(token, TokenUser, nullptr, 0, &length);
    std::vector<unsigned char> buffer(length ? length : 1);
    auto* user = reinterpret_cast<TOKEN_USER*>(buffer.data());
    const bool haveSid = length != 0 && GetTokenInformation(token, TokenUser, user, length, &length);
    CloseHandle(token);
    if (!haveSid) {
        if (error) {
            *error = QStringLiteral("Could not read the current user SID.");
        }
        return false;
    }

    LPWSTR text = nullptr;
    if (!ConvertSidToStringSidW(user->User.Sid, &text)) {
        if (error) {
            *error = QStringLiteral("Could not read the current user SID.");
        }
        return false;
    }
    const QString name = QStringLiteral("sapie-%1-local").arg(QString::fromWCharArray(text));
    LocalFree(text);

    QByteArray payload;
    for (const QString& argument : arguments) {
        payload.append(argument.toUtf8());
        payload.append('\0');
    }
    if (payload.isEmpty() || payload.size() > 65536) {
        if (error) {
            *error = QStringLiteral("The launch arguments are too large.");
        }
        return false;
    }

    QLocalSocket socket;
    socket.connectToServer(name, QIODevice::WriteOnly);
    if (!socket.waitForConnected(500) || !socket.write(payload)
        || !socket.waitForBytesWritten(500)) {
        if (error) {
            *error = QStringLiteral("No running launcher answered the single-instance pipe.");
        }
        return false;
    }
    return true;
}

void WindowsPlatform::onNewLocalConnection()
{
    auto* server = static_cast<QLocalServer*>(m_localServer);
    if (!server) {
        return;
    }
    while (server->hasPendingConnections()) {
        QLocalSocket* socket = server->nextPendingConnection();
        auto buffer = std::make_shared<QByteArray>();
        connect(socket, &QLocalSocket::readyRead, this, [this, socket, buffer]() {
            buffer->append(socket->readAll());
            if (buffer->size() > 65536) {
                socket->disconnectFromServer();
                return;
            }
            if (buffer->isEmpty() || buffer->constLast() != '\0') {
                return;
            }

            QStringList arguments;
            int start = 0;
            for (int index = 0; index < buffer->size(); ++index) {
                if (buffer->at(index) == '\0') {
                    arguments.append(QString::fromUtf8(buffer->constData() + start, index - start));
                    start = index + 1;
                }
            }
            emit secondInstanceLaunched(arguments);
            socket->disconnectFromServer();
        });
    }
}

}
