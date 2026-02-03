package router

import (
	"net"
	"os"

	"github.com/oschwald/geoip2-golang"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// GeoIPMatcher GeoIP规则匹配器
type GeoIPMatcher struct {
	db       *geoip2.Reader
	dbPath   string
	enabled  bool
	dbLoaded bool
}

// NewGeoIPMatcher 创建GeoIP匹配器
func NewGeoIPMatcher(dbPath string) *GeoIPMatcher {
	matcher := &GeoIPMatcher{
		dbPath:  dbPath,
		enabled: dbPath != "",
	}

	if matcher.enabled {
		if err := matcher.loadDatabase(); err != nil {
			logger.Warn("failed to load GeoIP database, GeoIP rules will be disabled",
				"path", dbPath,
				"error", err)
			matcher.enabled = false
		}
	} else {
		logger.Info("GeoIP database not configured, GeoIP rules disabled")
	}

	return matcher
}

// loadDatabase 加载GeoIP数据库
func (m *GeoIPMatcher) loadDatabase() error {
	// 检查文件是否存在
	if _, err := os.Stat(m.dbPath); os.IsNotExist(err) {
		return err
	}

	// 打开数据库
	db, err := geoip2.Open(m.dbPath)
	if err != nil {
		return err
	}

	m.db = db
	m.dbLoaded = true
	logger.Info("GeoIP database loaded successfully", "path", m.dbPath)
	return nil
}

// MatchGeoIP 匹配IP的国家代码
func (m *GeoIPMatcher) MatchGeoIP(ipStr, countryCode string) bool {
	if !m.enabled || !m.dbLoaded {
		logger.Debug("GeoIP matcher disabled or database not loaded")
		return false
	}

	// 解析IP地址
	ip := net.ParseIP(ipStr)
	if ip == nil {
		logger.Debug("invalid IP address for GeoIP lookup",
			"ip", ipStr)
		return false
	}

	// 查询国家信息
	record, err := m.db.Country(ip)
	if err != nil {
		logger.Debug("GeoIP lookup failed",
			"ip", ipStr,
			"error", err)
		return false
	}

	// 比较国家代码
	matched := record.Country.IsoCode == countryCode

	if matched {
		logger.Debug("GeoIP rule matched",
			"ip", ipStr,
			"country", record.Country.IsoCode,
			"pattern", countryCode)
	}

	return matched
}

// GetCountryCode 获取IP的国家代码
func (m *GeoIPMatcher) GetCountryCode(ipStr string) (string, error) {
	if !m.enabled || !m.dbLoaded {
		return "", nil
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", nil
	}

	record, err := m.db.Country(ip)
	if err != nil {
		return "", err
	}

	return record.Country.IsoCode, nil
}

// Close 关闭GeoIP数据库
func (m *GeoIPMatcher) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// IsEnabled 检查GeoIP是否可用
func (m *GeoIPMatcher) IsEnabled() bool {
	return m.enabled && m.dbLoaded
}
