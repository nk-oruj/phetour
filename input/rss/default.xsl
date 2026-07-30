<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:output method="xml" omit-xml-declaration="yes"/>

    <xsl:template match="/rss-item">
        <xsl:if test="@type != 'new-post' and @type != 'new-tag' and @type != 'updated-tag'">
            <xsl:message terminate="yes">Unknown RSS item type</xsl:message>
        </xsl:if>
        <rss-content>
            <title>
                <xsl:choose>
                    <xsl:when test="@type = 'new-post'">
                        <xsl:text>New post: </xsl:text>
                    </xsl:when>
                    <xsl:when test="@type = 'new-tag'">
                        <xsl:text>New tag: </xsl:text>
                    </xsl:when>
                    <xsl:when test="@type = 'updated-tag'">
                        <xsl:text>Updated tag: </xsl:text>
                    </xsl:when>
                </xsl:choose>
                <xsl:value-of select="document/meta/title/@value"/>
            </title>
            <description>
                <xsl:choose>
                    <xsl:when test="@type = 'new-post'">
                        <p><xsl:value-of select="document/body/text[1]"/></p>
                    </xsl:when>
                    <xsl:when test="@type = 'new-tag'">
                        <p>Posts currently listed under this tag:</p>
                        <ul>
                            <xsl:for-each select="document/body/link">
                                <li><a href="{concat(/rss-item/@site-url, @href)}"><xsl:value-of select="."/></a></li>
                            </xsl:for-each>
                        </ul>
                    </xsl:when>
                    <xsl:when test="@type = 'updated-tag'">
                        <p>Posts currently listed under this tag:</p>
                        <ul>
                            <xsl:for-each select="document/body/link">
                                <li><a href="{concat(/rss-item/@site-url, @href)}"><xsl:value-of select="."/></a></li>
                            </xsl:for-each>
                        </ul>
                    </xsl:when>
                </xsl:choose>
            </description>
            <category><xsl:value-of select="substring-after(@type, '-')"/></category>
        </rss-content>
    </xsl:template>
</xsl:stylesheet>
