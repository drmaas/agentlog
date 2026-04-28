# AgentLog installer for Windows
# Usage: powershell -ExecutionPolicy Bypass -Command "& { $(irm https://raw.githubusercontent.com/drmaas/agentlog/main/scripts/install.ps1) }"

param(
    [string]$Version = "latest",
    [string]$BinDir = "$env:APPDATA\agentlog\bin",
    [string]$Repo = "drmaas/agentlog"
)

function Get-Architecture {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture
    switch ($arch) {
        "X64" { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "Unsupported architecture: $arch" }
    }
}

function Download-File {
    param([string]$Url, [string]$OutPath)
    
    Write-Host "Downloading $Url"
    try {
        $ProgressPreference = 'SilentlyContinue'
        Invoke-WebRequest -Uri $Url -OutFile $OutPath -ErrorAction Stop
        if (-not (Test-Path $OutPath) -or (Get-Item $OutPath).Length -eq 0) {
            throw "Downloaded file is empty or missing"
        }
    } catch {
        Write-Error "Failed to download: $_"
        return $false
    }
    return $true
}

function Install-FromArchive {
    param([string]$Url, [string]$WorkDir)
    
    $archive = "$WorkDir\agentlog.zip"
    
    if (-not (Download-File $Url $archive)) {
        return $false
    }
    
    try {
        Expand-Archive -Path $archive -DestinationPath $WorkDir -Force -ErrorAction Stop
    } catch {
        Write-Error "Failed to extract archive: $_"
        return $false
    }
    
    $exe = "$WorkDir\agentlog.exe"
    if (-not (Test-Path $exe)) {
        Write-Error "agentlog.exe not found in archive"
        return $false
    }
    
    if (-not (Test-Path $BinDir)) {
        New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    }
    
    Copy-Item $exe "$BinDir\agentlog.exe" -Force
    return $true
}

function Install-FromGo {
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if (-not $goCmd) {
        Write-Error "No release artifact found and Go is not installed"
        exit 1
    }
    
    Write-Host "Falling back to go install"
    $goVersion = if ($Version -eq "latest") { "latest" } else { $Version }
    
    $env:GOBIN = $BinDir
    & go install "github.com/drmaas/agentlog/cmd/agentlog@$goVersion"
}

function Main {
    $arch = Get-Architecture
    $asset = "agentlog_windows_${arch}.zip"
    
    $baseUrl = if ($Version -eq "latest") {
        "https://github.com/$Repo/releases/latest/download"
    } else {
        "https://github.com/$Repo/releases/download/$Version"
    }
    
    $workDir = New-TemporaryFile | ForEach-Object { Remove-Item $_; New-Item -ItemType Directory -Path $_ }
    
    if (Install-FromArchive "$baseUrl/$asset" $workDir) {
        Write-Host "Installed agentlog to $BinDir\agentlog.exe"
    } else {
        Install-FromGo
        Write-Host "Installed agentlog to $BinDir\agentlog.exe"
    }
    
    Remove-Item $workDir -Recurse -Force -ErrorAction SilentlyContinue
    
    $pathDirs = [Environment]::GetEnvironmentVariable("PATH", "User") -split ";"
    if ($pathDirs -notcontains $BinDir) {
        Write-Host "Note: Add $BinDir to your PATH to run agentlog directly."
        Write-Host "Tip: setx PATH `"%PATH%;$BinDir`""
    }
}

Main
