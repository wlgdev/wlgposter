package tv.wlg.app.template;

import lombok.extern.slf4j.Slf4j;

@Slf4j
public class Application {
    public static void main(String[] args) {
        log.trace("TEST TRACE");
        log.debug("TEST DEBUG");
        log.info("Application ready");
        log.warn("TEST WARN");
        log.error("TEST ERROR");
    }
}
