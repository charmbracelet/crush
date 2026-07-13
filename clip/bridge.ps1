[CmdletBinding()]
param(
    [string]$ConfigPath = (Join-Path $PSScriptRoot "config.json")
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$config = Get-Content -Raw -LiteralPath $ConfigPath | ConvertFrom-Json
$token = (Get-Content -Raw -LiteralPath $config.token_path).Trim()
$maxImageBytes = 5MB
$logPath = Join-Path $PSScriptRoot "bridge.log"
$pidPath = Join-Path $PSScriptRoot "bridge.pid"

function Write-BridgeLog {
    param([string]$Message)
    Add-Content -LiteralPath $logPath -Value "$(Get-Date -Format o) $Message"
}

function Write-HttpResponse {
    param(
        [System.Net.Sockets.NetworkStream]$Stream,
        [int]$Status,
        [string]$Reason,
        [byte[]]$Body = [byte[]]::new(0),
        [string]$ContentType = "text/plain; charset=utf-8",
        [hashtable]$Headers = @{}
    )

    $headerText = "HTTP/1.1 $Status $Reason`r`nContent-Type: $ContentType`r`nContent-Length: $($Body.Length)`r`nConnection: close`r`n"
    foreach ($entry in $Headers.GetEnumerator()) {
        $headerText += "$($entry.Key): $($entry.Value)`r`n"
    }
    $headerText += "`r`n"
    $headerBytes = [Text.Encoding]::ASCII.GetBytes($headerText)
    $Stream.Write($headerBytes, 0, $headerBytes.Length)
    if ($Body.Length -gt 0) {
        $Stream.Write($Body, 0, $Body.Length)
    }
    $Stream.Flush()
}

function Read-BoundedLine {
    param(
        [IO.StreamReader]$Reader,
        [int]$MaxLength
    )

    $builder = [Text.StringBuilder]::new()
    while ($true) {
        $value = $Reader.Read()
        if ($value -lt 0) {
            if ($builder.Length -eq 0) {
                return $null
            }
            break
        }

        $char = [char]$value
        if ($char -eq "`n") {
            break
        }
        if ($char -eq "`r") {
            continue
        }
        if ($builder.Length -ge $MaxLength) {
            throw "HTTP line exceeds $MaxLength characters."
        }
        [void]$builder.Append($char)
    }
    return $builder.ToString()
}

function Get-ClipboardPng {
    if (-not [Windows.Forms.Clipboard]::ContainsImage()) {
        return $null
    }

    $image = [Windows.Forms.Clipboard]::GetImage()
    if ($null -eq $image) {
        return $null
    }

    try {
        $memory = [IO.MemoryStream]::new()
        try {
            $image.Save($memory, [Drawing.Imaging.ImageFormat]::Png)
            return $memory.ToArray()
        }
        finally {
            $memory.Dispose()
        }
    }
    finally {
        $image.Dispose()
    }
}

$listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, [int]$config.local_port)
[IO.File]::WriteAllText($pidPath, [string]$PID)

try {
    $listener.Start()
    Write-BridgeLog "clipboard bridge listening on 127.0.0.1:$($config.local_port)"

    :acceptLoop while ($true) {
        $client = $listener.AcceptTcpClient()
        try {
            $client.ReceiveTimeout = 5000
            $client.SendTimeout = 5000
            $stream = $client.GetStream()
            $stream.ReadTimeout = 5000
            $stream.WriteTimeout = 5000
            $reader = [IO.StreamReader]::new($stream, [Text.Encoding]::ASCII, $false, 4096, $true)
            $requestLine = Read-BoundedLine -Reader $reader -MaxLength 8192
            if ([string]::IsNullOrWhiteSpace($requestLine)) {
                continue
            }

            $headers = @{}
            $headerChars = 0
            while ($true) {
                $line = Read-BoundedLine -Reader $reader -MaxLength 8192
                if ([string]::IsNullOrEmpty($line)) {
                    break
                }
                $headerChars += $line.Length
                if ($headerChars -gt 32768) {
                    Write-HttpResponse -Stream $stream -Status 431 -Reason "Request Header Fields Too Large"
                    continue acceptLoop
                }
                $separator = $line.IndexOf(":")
                if ($separator -gt 0) {
                    $name = $line.Substring(0, $separator).Trim().ToLowerInvariant()
                    $headers[$name] = $line.Substring($separator + 1).Trim()
                }
            }

            $parts = $requestLine.Split(" ")
            $method = if ($parts.Length -gt 0) { $parts[0] } else { "" }
            $path = if ($parts.Length -gt 1) { $parts[1].Split("?")[0] } else { "" }

            if ($method -ne "GET") {
                Write-HttpResponse -Stream $stream -Status 405 -Reason "Method Not Allowed"
                continue
            }

            if ($path -eq "/health") {
                $body = [Text.Encoding]::UTF8.GetBytes('{"status":"ok"}')
                Write-HttpResponse -Stream $stream -Status 200 -Reason "OK" -Body $body -ContentType "application/json"
                continue
            }

            if ($path -ne "/v1/image") {
                Write-HttpResponse -Stream $stream -Status 404 -Reason "Not Found"
                continue
            }

            if ($headers["authorization"] -ne "Bearer $token") {
                Write-HttpResponse -Stream $stream -Status 401 -Reason "Unauthorized"
                continue
            }

            $imageBytes = Get-ClipboardPng
            if ($null -eq $imageBytes -or $imageBytes.Length -eq 0) {
                Write-HttpResponse -Stream $stream -Status 204 -Reason "No Content"
                continue
            }
            if ($imageBytes.Length -gt $maxImageBytes) {
                Write-HttpResponse -Stream $stream -Status 413 -Reason "Content Too Large"
                continue
            }

            $sha = [Security.Cryptography.SHA256]::Create()
            try {
                $digest = ([BitConverter]::ToString($sha.ComputeHash($imageBytes))).Replace("-", "").ToLowerInvariant()
            }
            finally {
                $sha.Dispose()
            }

            Write-HttpResponse -Stream $stream -Status 200 -Reason "OK" -Body $imageBytes -ContentType "image/png" -Headers @{ "X-Content-SHA256" = $digest }
        }
        catch [IO.IOException] {
            Write-BridgeLog "request timed out or disconnected: $($_.Exception.Message)"
        }
        catch {
            if ($null -ne $stream) {
                try {
                    Write-HttpResponse -Stream $stream -Status 400 -Reason "Bad Request"
                }
                catch {
                }
            }
            Write-BridgeLog "request failed: $($_.Exception.Message)"
        }
        finally {
            if ($null -ne $client) {
                $client.Dispose()
            }
        }
    }
}
finally {
    $listener.Stop()
    Remove-Item -LiteralPath $pidPath -Force -ErrorAction SilentlyContinue
    Write-BridgeLog "clipboard bridge stopped"
}
