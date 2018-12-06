FROM microsoft/windowsservercore:1709

RUN cmd.exe /C net users /ADD vcap /passwordreq:no /expires:never && runas /user:vcap whoami
RUN cmd.exe /C net accounts /maxpwage:UNLIMITED

RUN powershell.exe -Command \
  $ErrorActionPreference = 'Stop'; \
  \
  Add-WindowsFeature Web-Webserver, \
    Web-WebSockets, \
    Web-WHC, \
    Web-ASP, \
    Web-ASP-Net45

COPY Git-*-64-bit.exe /git-setup.exe
RUN C:\git-setup.exe /SILENT /NORESTART
RUN del /F C:\git-setup.exe

COPY tar-*.exe /Windows/tar.exe

COPY rewrite*.msi /Windows/rewrite.msi
RUN msiexec /i C:\Windows\rewrite.msi /qn /quiet

COPY vc_redist.x64.exe /vc_redist.x64.exe
RUN cmd.exe /s /c "c:\vc_redist.x64.exe /install /passive /norestart /wait"
RUN del /F "c:\vc_redist.x64.exe"

RUN powershell.exe -command "remove-windowsfeature -name 'windows-defender-features'"

RUN powershell.exe -command \
  $svs=('AppHostSvc', 'MSDTC', 'TermService', 'WAS', 'dhcp', 'diagtrack', 'w3svc', 'winrm', 'RemoteRegistry'); \
  foreach ($name in $svs) { Set-Service -Name $name -StartupType Disabled }

# Set to manual start - won't start unless needed (usually first net use)
RUN powershell.exe -command Set-Service -Name lmhosts -StartupType Manual

# Set to manual start - won't start unless needed (usually first DNS lookup)
RUN powershell.exe -command "Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Services\dnscache' -Name Start -Value 3"

# Minimize all cache TTLs to limit effect of service
RUN powershell.exe -command "Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Services\Dnscache\Parameters' -Name MaxCacheTtl -Value 0 -Type DWord"
RUN powershell.exe -command "Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Services\Dnscache\Parameters' -Name MaxNegativeCacheTtl -Value 0 -Type DWord"

# Ignore local hosts file - blank by defautl, all lookups should go to network
RUN powershell.exe -command "Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Services\Dnscache\Parameters' -Name UseHostsFile -Value 0 -Type DWord"

# Disable NETBIOS on all itnerfaces (needs confirmation in container-test)
RUN powershell.exe -command "Invoke-CimMethod -Query 'SELECT * FROM Win32_NetworkAdapterConfiguration WHERE IPEnabled=1' -MethodName SetTcpipNetbios -Arguments @{TcpipNetbiosOptions=[uint32]2}"

# Disable WINS on all itnerfaces (needs confirmation in container-test)
RUN powershell.exe -command "Invoke-CimMethod -ClassName Win32_NetworkAdapterConfiguration -MethodName EnableWINS -Arguments @{DNSEnabledForWINSResolution = $false; WINSEnableLMHostsLookup = $false}"

# Disable LMHOSTS lookup (needs confirmation in container-test)
RUN powershell.exe -command "Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Services\NetBT\Parameters' -Name EnableLMHOSTS -Value 0 -Type DWord"
