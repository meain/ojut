# RAUS

**Record Audio Until Silence**

Here is how I use it with Hammerspon to enable Whisper based transcription to type.

``` lua
function transcribeAudio()
    local task = hs.task.new("/bin/sh", function(exitCode, output, stdErr)
        if exitCode ~= 0 then
            hs.alert.show("Transcription failed: " .. (stdErr or "Unknown error"))
            return
        end

        output = utils.trim(output)

        if output == "." then
            -- Just stopped previous one
            return
        elseif output == "" then
            hs.alert.show("Speak up")
        else
            hs.eventtap.keyStrokes(output)
        end
    end, {"-c", ",transcribe-audio"})

    task:start()
end

hs.hotkey.bind(fkey, ";", transcribeAudio)
```

Here is the ,transcribe-audio script:

``` shell
#!/bin/sh

set -e

if pgrep -x "raus" > /dev/null; then
    kill -s HUP $(pgrep -x "raus")
    echo .
    exit
fi

raus |
    whisper-cpp -m "$HOME/dev/src/record-audio-until-silence/ggml-medium.en.bin" -f - -np -nt |
    tr -d '\n' |
    sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
```