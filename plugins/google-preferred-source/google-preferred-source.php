<?php
/**
 * Plugin Name: Google Preferred Source Publisher Widget
 * Plugin URI:  https://hirelyuk.com
 * Description: Appends an ultra-premium "Add us as a Preferred Source on Google" widget/button to every blog post and custom post type for enhanced visibility in Google Top Stories, AI Overviews, and AI Mode.
 * Version:     1.0.0
 * Author:      HirelyUK & AI Engineering Team
 * Author URI:  https://hirelyuk.com
 * License:     GPL-2.0+
 * Text Domain: google-preferred-source
 */

if (!defined('ABSPATH')) {
    exit; // Exit if accessed directly
}

class GooglePreferredSourcePlugin {

    const OPTION_KEY = 'gps_settings';

    public function __construct() {
        // Content Hook
        add_filter('the_content', array($this, 'append_preferred_source_button'), 30);

        // Shortcode
        add_shortcode('google_preferred_source', array($this, 'render_shortcode'));

        // Admin Menu & Settings
        add_action('admin_menu', array($this, 'register_admin_menu'));
        add_action('admin_init', array($this, 'register_settings'));
    }

    /**
     * Get plugin settings with sensible defaults.
     */
    public static function get_settings() {
        $defaults = array(
            'domain'            => wp_parse_url(home_url(), PHP_URL_HOST),
            'pre_text'          => 'Enjoying our content? Make us a preferred source on Google.',
            'sub_text'          => 'Follow us to see our latest articles, guides, and updates prioritized in your Google Top Stories, AI Overviews & AI Mode.',
            'button_text'       => 'Add us as a Preferred Source on Google',
            'post_types'        => array('post'),
            'button_style'      => 'google_modern',
            'show_subtext_badge'=> 1,
            'badge_text'        => 'Official Google Search Feature',
        );

        $options = get_option(self::OPTION_KEY, array());
        return wp_parse_args($options, $defaults);
    }

    /**
     * Render the Google Preferred Source Card HTML.
     */
    public static function render_widget_html($custom_args = array()) {
        $settings = self::get_settings();
        $domain = !empty($custom_args['domain']) ? sanitize_text_field($custom_args['domain']) : $settings['domain'];
        $clean_domain = preg_replace('/^www\./', '', $domain);
        $google_url = 'https://www.google.com/preferences/source?q=' . rawurlencode($clean_domain);

        $pre_text = !empty($custom_args['title']) ? sanitize_text_field($custom_args['title']) : $settings['pre_text'];
        $sub_text = !empty($custom_args['desc']) ? sanitize_text_field($custom_args['desc']) : $settings['sub_text'];
        $btn_text = !empty($custom_args['button']) ? sanitize_text_field($custom_args['button']) : $settings['button_text'];

        ob_start();
        ?>
        <!-- Google Preferred Source Widget Start -->
        <div class="google-pref-source-wrapper" style="margin: 32px 0 24px 0; clear: both;">
            <style>
                .gps-card {
                    position: relative;
                    background: linear-gradient(135deg, #ffffff 0%, #f8faff 50%, #f0f7ff 100%);
                    border: 1.5px solid #dbeafe;
                    border-radius: 16px;
                    padding: 24px 28px;
                    box-shadow: 0 4px 20px -2px rgba(10, 102, 194, 0.08), 0 1px 3px rgba(0, 0, 0, 0.03);
                    text-align: left;
                    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen, Ubuntu, Cantarell, "Open Sans", "Helvetica Neue", sans-serif;
                    box-sizing: border-box;
                    overflow: hidden;
                    transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
                }
                .gps-card:hover {
                    border-color: #93c5fd;
                    box-shadow: 0 8px 30px -4px rgba(10, 102, 194, 0.15), 0 2px 6px rgba(0, 0, 0, 0.04);
                    transform: translateY(-2px);
                }
                .gps-badge {
                    display: inline-flex;
                    align-items: center;
                    gap: 6px;
                    background: #ffffff;
                    border: 1px solid #e2e8f0;
                    box-shadow: 0 1px 2px rgba(0,0,0,0.05);
                    padding: 3px 10px;
                    border-radius: 9999px;
                    font-size: 11px;
                    font-weight: 700;
                    color: #1e293b;
                    margin-bottom: 10px;
                    text-transform: uppercase;
                    letter-spacing: 0.04em;
                }
                .gps-title {
                    font-size: 18px;
                    font-weight: 700;
                    color: #0f172a;
                    line-height: 1.35;
                    margin: 0 0 8px 0;
                }
                .gps-desc {
                    font-size: 14px;
                    color: #475569;
                    line-height: 1.55;
                    margin: 0 0 18px 0;
                }
                .gps-action-row {
                    display: flex;
                    align-items: center;
                    flex-wrap: wrap;
                    gap: 14px;
                }
                .gps-btn {
                    display: inline-flex;
                    align-items: center;
                    justify-content: center;
                    gap: 10px;
                    background: #ffffff;
                    color: #1a73e8;
                    border: 1.5px solid #dadce0;
                    padding: 10px 22px;
                    border-radius: 9999px;
                    font-weight: 700;
                    font-size: 14px;
                    text-decoration: none !important;
                    box-shadow: 0 2px 6px rgba(60, 64, 67, 0.12), 0 1px 2px rgba(60, 64, 67, 0.08);
                    transition: all 0.25s ease;
                    cursor: pointer;
                    line-height: 1.2;
                }
                .gps-btn:hover {
                    background: #f8fafd;
                    border-color: #1a73e8;
                    color: #174ea6;
                    box-shadow: 0 4px 12px rgba(26, 115, 232, 0.22);
                    transform: translateY(-1px);
                    text-decoration: none !important;
                }
                .gps-btn svg {
                    flex-shrink: 0;
                }
                .gps-subtext {
                    font-size: 12px;
                    font-weight: 600;
                    color: #166534;
                    display: inline-flex;
                    align-items: center;
                    gap: 5px;
                }
                @media (max-width: 600px) {
                    .gps-card {
                        padding: 18px 16px;
                    }
                    .gps-title {
                        font-size: 16px;
                    }
                    .gps-action-row {
                        flex-direction: column;
                        align-items: stretch;
                        gap: 10px;
                    }
                    .gps-btn {
                        width: 100%;
                        justify-content: center;
                        padding: 12px 16px;
                    }
                    .gps-subtext {
                        justify-content: center;
                    }
                }
            </style>
            <div class="gps-card">
                <?php if (!empty($settings['show_subtext_badge'])): ?>
                <div class="gps-badge">
                    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
                        <path fill="#4285F4" d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.82-2.4 3.68v3.05h3.88c2.27-2.09 3.665-5.17 3.665-9.17z"/>
                        <path fill="#34A853" d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.88-3.05c-1.08.72-2.45 1.16-4.05 1.16-3.12 0-5.77-2.1-6.72-4.93H1.25v3.15C3.26 21.36 7.34 24 12 24z"/>
                        <path fill="#FBBC05" d="M5.28 14.27c-.25-.72-.38-1.49-.38-2.27s.13-1.55.38-2.27V6.58H1.25C.45 8.18 0 9.98 0 12s.45 3.82 1.25 5.42l4.03-3.15z"/>
                        <path fill="#EA4335" d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.34 0 3.26 2.64 1.25 6.58l4.03 3.15c.95-2.83 3.6-4.98 6.72-4.98z"/>
                    </svg>
                    <span><?php echo esc_html($settings['badge_text']); ?></span>
                </div>
                <?php endif; ?>

                <div class="gps-title"><?php echo esc_html($pre_text); ?></div>
                
                <?php if (!empty($sub_text)): ?>
                <div class="gps-desc"><?php echo esc_html($sub_text); ?></div>
                <?php endif; ?>

                <div class="gps-action-row">
                    <a href="<?php echo esc_url($google_url); ?>" 
                       class="gps-btn" 
                       target="_blank" 
                       rel="noopener noreferrer" 
                       title="<?php echo esc_attr($btn_text); ?>">
                        <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
                            <path fill="#4285F4" d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.82-2.4 3.68v3.05h3.88c2.27-2.09 3.665-5.17 3.665-9.17z"/>
                            <path fill="#34A853" d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.88-3.05c-1.08.72-2.45 1.16-4.05 1.16-3.12 0-5.77-2.1-6.72-4.93H1.25v3.15C3.26 21.36 7.34 24 12 24z"/>
                            <path fill="#FBBC05" d="M5.28 14.27c-.25-.72-.38-1.49-.38-2.27s.13-1.55.38-2.27V6.58H1.25C.45 8.18 0 9.98 0 12s.45 3.82 1.25 5.42l4.03-3.15z"/>
                            <path fill="#EA4335" d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.34 0 3.26 2.64 1.25 6.58l4.03 3.15c.95-2.83 3.6-4.98 6.72-4.98z"/>
                        </svg>
                        <span><?php echo esc_html($btn_text); ?></span>
                        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path>
                            <polyline points="15 3 21 3 21 9"></polyline>
                            <line x1="10" y1="14" x2="21" y2="3"></line>
                        </svg>
                    </a>
                    <span class="gps-subtext">
                        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                            <polyline points="20 6 9 17 4 12"></polyline>
                        </svg>
                        Takes 5 seconds &bull; 1-click confirmation on Google
                    </span>
                </div>
            </div>
        </div>
        <!-- Google Preferred Source Widget End -->
        <?php
        return ob_get_clean();
    }

    /**
     * Shortcode handler: [google_preferred_source]
     */
    public function render_shortcode($atts) {
        $atts = shortcode_atts(array(
            'title'  => '',
            'desc'   => '',
            'button' => '',
            'domain' => '',
        ), $atts, 'google_preferred_source');

        return self::render_widget_html($atts);
    }

    /**
     * Append to single post content via the_content filter.
     */
    public function append_preferred_source_button($content) {
        if (is_admin() || is_feed() || !is_singular()) {
            return $content;
        }

        $settings = self::get_settings();
        $post_type = get_post_type();

        if (!in_array($post_type, (array)$settings['post_types'])) {
            return $content;
        }

        // If post already has the shortcode, skip auto injection
        if (has_shortcode($content, 'google_preferred_source')) {
            return $content;
        }

        return $content . self::render_widget_html();
    }

    /**
     * Register Settings Menu in WordPress Admin.
     */
    public function register_admin_menu() {
        add_options_page(
            __('Google Preferred Source', 'google-preferred-source'),
            __('Google Preferred Source', 'google-preferred-source'),
            'manage_options',
            'google-preferred-source',
            array($this, 'render_admin_page')
        );
    }

    public function register_settings() {
        register_setting('gps_settings_group', self::OPTION_KEY);
    }

    /**
     * Admin Settings Page UI.
     */
    public function render_admin_page() {
        $settings = self::get_settings();
        ?>
        <div class="wrap">
            <h1><?php _e('Google Preferred Source Settings', 'google-preferred-source'); ?></h1>
            <p><?php _e('Configure the button that prompts readers to add your site as a Preferred Source on Google (boosting your visibility in Top Stories and AI Overviews).', 'google-preferred-source'); ?></p>

            <form method="post" action="options.php">
                <?php
                settings_fields('gps_settings_group');
                ?>
                <table class="form-table">
                    <tr>
                        <th scope="row"><label for="gps_domain"><?php _e('Publisher Domain', 'google-preferred-source'); ?></label></th>
                        <td>
                            <input name="gps_settings[domain]" type="text" id="gps_domain" value="<?php echo esc_attr($settings['domain']); ?>" class="regular-text" placeholder="example.com" />
                            <p class="description"><?php _e('The domain configured in Google for this publication (without https://).', 'google-preferred-source'); ?></p>
                        </td>
                    </tr>
                    <tr>
                        <th scope="row"><label for="gps_pre_text"><?php _e('Heading Title', 'google-preferred-source'); ?></label></th>
                        <td>
                            <input name="gps_settings[pre_text]" type="text" id="gps_pre_text" value="<?php echo esc_attr($settings['pre_text']); ?>" class="large-text" />
                        </td>
                    </tr>
                    <tr>
                        <th scope="row"><label for="gps_sub_text"><?php _e('Description Subtitle', 'google-preferred-source'); ?></label></th>
                        <td>
                            <textarea name="gps_settings[sub_text]" id="gps_sub_text" rows="2" class="large-text"><?php echo esc_textarea($settings['sub_text']); ?></textarea>
                        </td>
                    </tr>
                    <tr>
                        <th scope="row"><label for="gps_btn_text"><?php _e('Button Label', 'google-preferred-source'); ?></label></th>
                        <td>
                            <input name="gps_settings[button_text]" type="text" id="gps_btn_text" value="<?php echo esc_attr($settings['button_text']); ?>" class="regular-text" />
                        </td>
                    </tr>
                    <tr>
                        <th scope="row"><?php _e('Active Post Types', 'google-preferred-source'); ?></th>
                        <td>
                            <?php
                            $available_types = get_post_types(array('public' => true), 'objects');
                            foreach ($available_types as $pt):
                                $checked = in_array($pt->name, (array)$settings['post_types']) ? 'checked="checked"' : '';
                            ?>
                                <label style="margin-right: 15px;">
                                    <input type="checkbox" name="gps_settings[post_types][]" value="<?php echo esc_attr($pt->name); ?>" <?php echo $checked; ?> />
                                    <?php echo esc_html($pt->label); ?>
                                </label>
                            <?php endforeach; ?>
                        </td>
                    </tr>
                </table>

                <?php submit_button(); ?>
            </form>

            <hr style="margin-top: 30px;" />
            <h3><?php _e('Live Widget Preview', 'google-preferred-source'); ?></h3>
            <div style="max-width: 720px;">
                <?php echo self::render_widget_html(); ?>
            </div>

            <hr style="margin-top: 30px;" />
            <h3><?php _e('Manual Shortcode Usage', 'google-preferred-source'); ?></h3>
            <p><?php _e('You can manually insert the widget into any page, post, or block with:', 'google-preferred-source'); ?></p>
            <code>[google_preferred_source]</code>
            <p><?php _e('Or override custom parameters on a specific post:', 'google-preferred-source'); ?></p>
            <code>[google_preferred_source title="Follow our breaking coverage" button="Add us on Google"]</code>
        </div>
        <?php
    }
}

new GooglePreferredSourcePlugin();
